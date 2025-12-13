package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/repository"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"context"
	"go.uber.org/zap"
	"log"
	"time"
)

type chatService struct {
	seqFactory  *SeqFactory
	pushService *PushService
	chatRepo    repository.IChatRepo
}

func NewChatService(sq *SeqFactory, ps *PushService, cr repository.IChatRepo) IChatService {
	return &chatService{
		seqFactory:  sq,
		pushService: ps,
		chatRepo:    cr,
	}
}

func (c *chatService) pushMsg(ctx context.Context, reply *ws.ReplyMsg) {
	pushCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := c.pushService.Push(pushCtx, reply); err != nil {
		zap.L().Error("消息推送失败",
			zap.Uint64("sender_id", reply.SenderID),
			zap.String("client_msg_id", reply.ClientMsgID),
			zap.Error(err))
		// TODO: 添加死信队列
	}
}

// HandleChatMsg 处理上行聊天消息(用户发送过来的消息): 整个“上行”流程：去重 -> 取号 -> 落库 -> (给客户端)返回 ACK 数据。
func (c *chatService) HandleChatMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// TODO：进行消息参数校验
	senderID := client.UserID
	// 1. 生成会话ID
	conversationID := util.GetConversationID(senderID, req.ReceiverID)

	// 2. 幂等性检查 TODO: 将幂等性检查和取号封装至一起（事务绑定）
	//isDup, err := c.seqFactory.CheckAndSetDedup(ctx, conversationID, req.ClientMsgID)
	//if err != nil {
	//	return err
	//}
	//if isDup {
	//	log.Println("消息重复")
	//	return nil
	//}
	//
	//// 3.取号：生成SeqID
	//seqID, err := c.seqFactory.GetNextSeqID(ctx, conversationID)
	//if err != nil {
	//	return err
	//}
	isDup, seqID, err := c.seqFactory.CheckAndSetDedupWithSeq(ctx, conversationID, req.ClientMsgID, time.Hour)
	if err != nil {
		zap.L().Error("消息去重失/取号失败", zap.Error(err))
	}
	if isDup {
		log.Println("消息重复")
		return nil
	}

	// 4. 消息落库
	createdAt := time.Now().UTC()
	msg := dao.MessageModel{
		ConversationID: conversationID,
		SeqID:          seqID,
		SenderID:       senderID,
		ReceiverID:     req.ReceiverID,
		Content:        req.Content,
		MsgType:        req.MsgType,
		MsgStatus:      util.MsgStatusRead,
		CreatedAt:      createdAt,
	}
	if err = util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		return c.chatRepo.Create(ctx, &msg)
	}); err != nil {
		return err
	}

	// 4. 异步调用下行推送服务（带重试+死信队列）
	util.SafeGo(func() {
		reply := &ws.ReplyMsg{
			Cmd:         ws.CmdChat,
			ClientMsgID: req.ClientMsgID,
			SeqID:       seqID,
			SenderID:    senderID,
			ReceiverID:  req.ReceiverID,
			Content:     req.Content,
			TimeStamp:   createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, reply)
	})

	// 5. 异步回复ACK（发送方）（主流程不阻塞，失败后重试）
	util.SafeGo(func() {
		ack := &ws.ReplyMsg{
			Cmd:         ws.CmdAck,
			ReceiverID:  req.SenderID,
			ClientMsgID: req.ClientMsgID,
			SeqID:       seqID,
			TimeStamp:   createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, ack)
	})

	return nil
}

// HandleAckMsg 服务端收到客户端的ACK消息处理方法
func (c *chatService) HandleAckMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// 对ack信息中AckID字段的信息进行处理
	//seqID := req.AckID
	// 1. 对MySQL中SeqID的消息标记为已读
	return nil
}

// HandleRevokeMsg 处理撤回消息
func (c *chatService) HandleRevokeMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	return nil
}

// HandleDeleteMsg 处理删除消息
func (c *chatService) HandleDeleteMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	return nil
}

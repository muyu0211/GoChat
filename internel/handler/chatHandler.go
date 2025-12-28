package handler

import (
	"GoChat/internel/model/dto"
	"GoChat/internel/service"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

/**
 * @Description: 处理用户请求的ws连接
 */

type ChatHandler struct {
	chatService service.IChatService
	userService service.IUserService
	syncService service.ISyncService
	pushService service.IPushService
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,                                       // 4196
	WriteBufferSize: 1024,                                       // 1124
	CheckOrigin:     func(r *http.Request) bool { return true }, // 允许跨域
}

func NewChatHandler(cs service.IChatService, us service.IUserService, ss service.ISyncService, ps service.IPushService) *ChatHandler {
	return &ChatHandler{
		chatService: cs,
		userService: us,
		syncService: ss,
		pushService: ps,
	}
}

// Connect 登录之后，客户端紧接着发送ws连接请求
func (ch *ChatHandler) Connect(c *gin.Context) {
	// 1. 判断用户登陆情况
	var userID uint64
	var ok bool
	ctx := c.Request.Context()
	v, exists := c.Get(util.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, util.NewResMsg("0", "登录已过期", nil))
		return
	}
	if userID, ok = v.(uint64); ok != true {
		c.JSON(http.StatusInternalServerError, util.NewResMsg("0", "登录已过期", nil))
		return
	}

	// 2. 从c中取出Writer和Request
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	// 获取 client 实例对象
	wsRouter := ws.NewWsRouter()
	wsRouter.Register(ws.CmdChat, ch.chatService.HandleChatMsg)
	wsRouter.Register(ws.CmdAck, ch.chatService.HandleAckMsg)
	wsRouter.Register(ws.CmdRevoke, ch.chatService.HandleRevokeMsg)

	client := ws.NewClient(conn, userID, wsRouter, func(userID uint64) {
		_ = ch.userService.UserOffline(c, userID)
	})

	// 执行用户上线操作
	err = ch.userService.UserOnline(ctx, userID)
	if err == nil {
		log.Printf("用户：%d 上线", userID)
	}

	// 注册 client
	ws.Manager.Register(client, userID)
	util.SafeGo(func() {
		client.ReadPump()
	})
	util.SafeGo(func() {
		client.WritePump()
	})
	util.SafeGo(func() {
		// 用户上线后进行消息同步
		ch.syncService.Sync(c, userID)
	})
}

func (ch *ChatHandler) GetAllClient(c *gin.Context) {
	userIDs := ws.Manager.GetAllClient()
	c.JSON(http.StatusOK, util.NewResMsg("1", "成功", userIDs))
}

// GetUserConverse 获取用户的会话列表（用于客户端进行消息同步）
func (ch *ChatHandler) GetUserConverse(c *gin.Context) {
	var req dto.UserReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.NewResMsg("0", "参数错误", nil))
		return
	}

	if req.UserID == 0 {
		userIDAny, exist := c.Get(util.CtxUserIDKey)
		if !exist {
			c.JSON(http.StatusInternalServerError, util.NewResMsg("0", "用户ID获取失败", nil))
			return
		}
		var ok bool
		req.UserID, ok = userIDAny.(uint64)
		if !ok {
			c.JSON(http.StatusInternalServerError, util.NewResMsg("0", "用户ID获取失败", nil))
			return
		}
	}

	sessions, err := ch.syncService.GetSessions(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

// SyncConverse 同步用户某一会话中的消息
func (ch *ChatHandler) SyncConverse(c *gin.Context) {
	var req dto.GetMsgHistoryReq
	var ctx = c.Request.Context()
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == 0 {
		userID, exist := c.Get(util.CtxUserIDKey)
		if !exist {
			c.JSON(http.StatusInternalServerError, util.NewResMsg("0", "用户ID获取失败", nil))
			return
		}
		var ok bool
		req.UserID, ok = userID.(uint64)
		if !ok {
			c.JSON(http.StatusInternalServerError, util.NewResMsg("0", "用户ID获取失败", nil))
			return
		}
	}

	msgs, hasMore, err := ch.syncService.SyncConverse(ctx, req.UserID, req.ConversationID, req.LastAckID, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.NewResMsg("0", fmt.Sprintf("获取消息失败：%v", err), nil))
		return
	}

	var nextSeq = req.LastAckID
	if len(msgs) > 0 {
		nextSeq = msgs[len(msgs)-1].SeqID
	}
	c.JSON(http.StatusOK, util.NewResMsg("1", "成功", gin.H{
		"msgs":     msgs,
		"has_more": hasMore,
		"next_seq": nextSeq,
	}))
}

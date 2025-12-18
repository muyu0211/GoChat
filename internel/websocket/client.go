package websocket

import (
	"GoChat/pkg/util"
	"context"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"log"
	"time"
)

/**
管理客户端连接对象
*/

type Client struct {
	conn       *websocket.Conn     // 持有一个ws连接
	DataBuffer chan []byte         // 接收数据的缓冲管道
	UserID     uint64              // 连接持有者id
	wsRouter   *WsRouter           // 对于不同的消息的处理方法路由
	OnClose    func(userID uint64) // 用户下线的相关操作
}

func NewClient(conn *websocket.Conn, userID uint64, wr *WsRouter, onClose func(userID uint64)) *Client {
	return &Client{
		conn:       conn,
		DataBuffer: make(chan []byte, 512),
		UserID:     userID,
		wsRouter:   wr,
		OnClose:    onClose,
	}
}

// Close 关闭ws连接
func (c *Client) Close() {
	_ = c.conn.Close()
}

// WritePump 写协程：专门负责把 Send 通道里的数据写给前端（写给ws连接）
func (c *Client) WritePump() {
	defer func() {
		c.Close()
	}()
	for {
		select {
		case message, ok := <-c.DataBuffer:
			if !ok { // 如果 channel 被关闭 (ok == false)，说明 ReadPump 那边触发了销毁流程
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			// 设置写超时，防止网络卡死导致协程泄露
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				zap.L().Error("获取写入器失败", zap.Error(err))
				return
			}
			_, err = w.Write(message)
			if err != nil {
				zap.L().Error("写入数据失败", zap.Error(err))
				return
			}

			// 如果队列里还有消息，一次性发完优化网络
			n := len(c.DataBuffer)
			for i := 0; i < n; i++ {
				_, err = w.Write(<-c.DataBuffer)
				if err != nil {
					zap.L().Error("写入数据失败", zap.Error(err))
					return
				}
			}

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

// ReadPump 读取消息（从ws连接中读取数据）
func (c *Client) ReadPump() {
	defer func() {
		Manager.Unregister(c.UserID)
		if c.OnClose != nil {
			c.OnClose(c.UserID)
		}
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			// 判断是否是意外关闭 (如果不是 1000 或 1001，则视为异常并打印日志)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				zap.L().Error("WebSocket 读取异常", zap.Error(err))
			} else {
				log.Println("WebSocket 已关闭")
			}
			return
		}

		var req SendMsg
		// 反序列化消息
		if err = util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
			return req.Deserialize(msg)
		}); err != nil {
			zap.L().Error("序列化消息失败", zap.Error(err))
			continue
		}
		//if err = req.Deserialize(msg); err != nil {
		//	reply = &ReplyMsg{
		//		ReceiverID: req.SenderID, // 服务器给发送方发送确认/拒绝消息，接收者ID为发送方本身
		//		Cmd:        CmdError,
		//		Content:    "序列化消息失败",
		//		SenderID:   c.UserID,
		//		TimeStamp:  req.TimeStamp,
		//	}
		//	replyByte, _ := reply.Serialize()
		//	c.DataBuffer <- replyByte
		//	continue
		//}

		// 处理消息
		err = c.wsRouter.Dispatch(context.Background(), c, &req)
		if err != nil {
			zap.L().Error("Handle Msg Error",
				zap.String("cmd", req.Cmd),
				zap.Error(err),
			)
		}
	}
}

func (c *Client) Ping() {

}

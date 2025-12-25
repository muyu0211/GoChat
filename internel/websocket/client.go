package websocket

import (
	"GoChat/pkg/util"
	"context"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

/**
管理客户端连接对象
*/

const (
	// WriteWait: 写超时时间 (服务端发给客户端消息的超时)
	writeWait = 10 * time.Second

	// PongWait: 等待客户端 Pong 的最大时间
	// 如果超过这个时间没收到 Pong，视为连接断开
	pongWait = 150 * time.Second

	// PingPeriod: 服务端发送 Ping 的频率
	// ⚠️ 必须小于 PongWait (例如 PongWait 的 90%)，否则还没发 Ping 就判定超时了
	pingPeriod = (pongWait * 9) / 10

	// MaxMessageSize: 允许的最大消息体积
	maxMessageSize = 512
)

type Client struct {
	conn       *websocket.Conn     // 持有一个 ws 连接
	DataBuffer chan []byte         // 接收数据的缓冲管道
	UserID     uint64              // 连接持有者 id
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
	// 服务端创建定时器，定期ping一下客户端
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.DataBuffer:
			// 如果 channel 被关闭 (ok == false)，说明 ReadPump 那边触发了销毁流程
			if !ok {
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
		case <-pingTicker.C:
			// 设置写超时(
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				zap.L().Error("设置写超时失败", zap.Error(err))
				return
			}

			// 发送 Ping 消息 (Control Message), 浏览器 WebSocket API 会自动回复 Pong，无需前端手动写代码
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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

	// 1. 设置最大读取大小，防止恶意大包攻击
	c.conn.SetReadLimit(maxMessageSize)

	// 2. 设置初始的读取死线 (pongWait秒内未收到消息则断开连接)
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		zap.L().Error("设置读超时失败", zap.Error(err))
		return
	}

	// 3. 设置 Pong 处理器
	c.conn.SetPongHandler(func(appData string) error {
		log.Println("续命 appData: ", appData)
		err = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			zap.L().Error("设置读超时失败", zap.Error(err))
			return err
		}
		return nil
	})

	for {
		// 默认情况下进行正常的消息读取
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

		// 参数校验
		if req.ReceiverID == 0 {
			zap.L().Error("ReceiverID is empty")
			continue
		}
		if req.ConversationID == "" {
			req.ConversationID = util.GetConversationID(c.UserID, req.ReceiverID)
		}
		req.SenderID = c.UserID
		err = c.wsRouter.Dispatch(context.Background(), c, &req)
		if err != nil {
			zap.L().Error("Handle Msg Error",
				zap.String("cmd", req.Cmd),
				zap.Error(err),
			)
		}
	}
}

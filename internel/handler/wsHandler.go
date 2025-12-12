package handler

import (
	"GoChat/internel/service"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

/**
 * @Description: 处理用户请求的ws连接
 */

type WsHandler struct {
	chatService service.IChatService
	userService service.IUserService
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4196,
	WriteBufferSize: 1124,
	CheckOrigin:     func(r *http.Request) bool { return true }, // 允许跨域
}

func NewWsHandler(cs service.IChatService, us service.IUserService) *WsHandler {
	return &WsHandler{
		chatService: cs,
		userService: us,
	}
}

// Connect 登录之后，客户端紧接着发送ws连接请求
func (wh *WsHandler) Connect(c *gin.Context) {
	// 1. 判断用户登陆情况
	var userID uint64
	var ok bool
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

	// 获取client实例对象
	wsRouter := ws.NewWsRouter()
	wsRouter.Register(ws.CmdChat, wh.chatService.HandleChatMsg)
	wsRouter.Register(ws.CmdAck, wh.chatService.HandleAckMsg)
	wsRouter.Register(ws.CmdRevoke, wh.chatService.HandleRevokeMsg)

	client := ws.NewClient(conn, userID, wsRouter, func(userID uint64) {
		_ = wh.userService.UserOffline(c, userID)
	})

	// 执行用户上线操作
	err = wh.userService.UserOnline(c, userID)
	if err == nil {
		log.Printf("用户：%d 上线", userID)
	}

	// 注册client
	ws.Manager.Register(client, userID)
	util.SafeGo(func() {
		client.ReadPump()
	})
	util.SafeGo(func() {
		client.WritePump()
	})
	util.SafeGo(func() {
		// TODO: 设置心跳保活
		//client.Heartbeat()
	})
}

func (wh *WsHandler) GetAllClient(c *gin.Context) {
	userIDs := ws.Manager.GetAllClient()
	c.JSON(http.StatusOK, util.NewResMsg("1", "成功", userIDs))
}

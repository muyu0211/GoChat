package websocket

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"sync"
)

/*
 * @Description: websocket 对于各种消息的操作的路由分发
 */

type HandlerFunc func(ctx context.Context, client *Client, req *SendMsg) error

type WsRouter struct {
	handlers map[string]HandlerFunc
	sync.RWMutex
}

func NewWsRouter() *WsRouter {
	return &WsRouter{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register 注册路由
func (r *WsRouter) Register(cmd string, handler HandlerFunc) {
	r.Lock()
	defer r.Unlock()
	r.handlers[cmd] = handler
}

// Dispatch 分发路由
func (r *WsRouter) Dispatch(ctx context.Context, client *Client, req *SendMsg) error {
	r.RLock()
	handler, ok := r.handlers[req.Cmd] // 获取对应操作的处理函数
	r.RUnlock()

	if !ok {
		zap.L().Warn("没有对应的处理函数", zap.String("cmd", req.Cmd))
		return errors.New("没有对应的处理函数")
	}
	return handler(ctx, client, req)
}

package service

import (
	ws "GoChat/internel/websocket"
	"context"
)

type IChatService interface {
	HandleChatMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleAckMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleRevokeMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleDeleteMsg(context.Context, *ws.Client, *ws.SendMsg) error
}

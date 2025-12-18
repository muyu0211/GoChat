package test

import (
	"GoChat/internel/websocket"
	"testing"
)

type PushPayLoad struct {
	Msg          []byte `json:"msg_data"`
	ReceiverID   uint64 `json:"receiver_id"`
	TargetServer string `json:"target_server"`
}

func TestWS(t *testing.T) {
	msg := websocket.ReplyMsg{
		Cmd:            "chat",
		ConversationID: "123",
		ClientMsgID:    "123",
		Flag:           0,
		Content:        "123123",
		SenderID:       1,
		ReceiverID:     2,
		TimeStamp:      3,
		SeqID:          4,
		AckID:          5,
	}
	msgBytes, err := msg.Serialize()
	if err != nil {
		t.Error(err)
	}
	t.Log(string(msgBytes))
}

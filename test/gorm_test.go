package test

import (
	"GoChat/internel/websocket"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
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

func TestTicker(t *testing.T) {
	t.Log("计时器测试")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	ticker := time.NewTicker(time.Second)
	start := time.Now()
	for {
		select {
		case <-signals:
			t.Log("测试退出")
		case <-ticker.C:
			func() {
				t.Log("计时器触发")
				t.Log(time.Now().Sub(start))
				time.Sleep(5 * time.Second)
			}()
		}
	}
}

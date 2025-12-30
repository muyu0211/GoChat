package test

import (
	"GoChat/config"
	"GoChat/internel/model/dao"
	"GoChat/internel/websocket"
	"GoChat/pkg/auth"
	"GoChat/pkg/db"
	"GoChat/pkg/logger"
	"GoChat/pkg/util"
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"gorm.io/gorm"
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

func TestGorm(t *testing.T) {
	// 初始化配置文件
	cfg := config.LoadConfig()

	// 服务启动
	//startServer()
	logger.StartLogger(cfg)
	db.StartMySQL(cfg)
	db.StartRedis(cfg)
	auth.StartJWT(cfg)
	util.StartAntsPool(cfg)

	dbs := db.GetDBS()
	var conversation dao.ConversationModel
	result := dbs.Master.Model(&dao.ConversationModel{}).WithContext(context.Background()).
		Select("last_seq_id").
		Where("owner_id = ? and conversation_id = ?", 1, "1_2").Find(&conversation)

	t.Log(errors.Is(result.Error, gorm.ErrRecordNotFound))
}

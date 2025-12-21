package mq

import "encoding/json"

type AckEvent struct {
	SenderID       uint64 `json:"sender_id"` // 发送者ID
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	TimeStamp      int64  `json:"time_stamp"`
	AckID          uint64 `json:"ack_id"`
}

func (msg *AckEvent) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *AckEvent) Deserialize(data []byte) error {
	return json.Unmarshal(data, msg)
}

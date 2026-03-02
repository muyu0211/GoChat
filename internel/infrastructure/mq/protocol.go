package mq

import "encoding/json"

const (
	GroupCreateKey = "im_group_create_events"
)

type AckEvent struct {
	SenderID       uint64 `json:"sender_id"` // 发送者ID
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	TimeStamp      int64  `json:"time_stamp"`
	AckID          uint64 `json:"ack_id"`
}

type GroupMsgEvent struct {
	Cmd         string   `json:"cmd"` // 消息类型：group_chat
	GroupID     uint64   `json:"group_id"`
	GroupKeyID  int64    `json:"group_key_id"` // 群组主键ID
	SenderID    uint64   `json:"sender_id"`
	MemberIDs   []uint64 `json:"member_ids"`
	Content     string   `json:"content"`
	TimeStamp   int64    `json:"time_stamp"`
	ClientMsgID string   `json:"client_msg_id"` // 客户端消息ID
	SeqID       uint64   `json:"seq_id"`
	MsgType     byte     `json:"msg_type"` // 消息类型
}

func (msg *AckEvent) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *AckEvent) Deserialize(data []byte) error {
	return json.Unmarshal(data, msg)
}

func (msg *GroupMsgEvent) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *GroupMsgEvent) Deserialize(data []byte) error {
	return json.Unmarshal(data, msg)
}

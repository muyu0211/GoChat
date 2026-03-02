package websocket

import "encoding/json"

/**

 */

const (
	CmdSingleChat = "single_chat"
	CmdGroupChat  = "group_chat"
	CmdSystem     = "system"
	CmdAck        = "ack"
	CmdRevoke     = "revoke"
	CmdDelete     = "delete"
	CmdError      = "error"
)

type Msg interface {
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
	GetCmd() string
	GetSeqID() uint64
	GetReceiverID() uint64
	GetSenderID() uint64
	GetConversationID() string
}

// SendMsg 客户端发送给服务器的	消息结构体
type SendMsg struct {
	Cmd            string `json:"cmd"`
	GroupKeyID     int64  `json:"group_key_id"`
	ConversationID string `json:"conversation_id"`
	ClientMsgID    string `json:"client_msg_id"` // 每条消息的全局唯一ID（客户端生成），可以是UUID, TODO: 区分网络重试和业务重试()
	Flag           uint8  `json:"flag"`
	Content        string `json:"content"`
	SenderID       uint64 `json:"sender_id"`
	ReceiverID     uint64 `json:"receiver_id"`
	TimeStamp      int64  `json:"time_stamp"` // 发送消息的时间戳
	MsgType        byte   `json:"msg_type"`   // 消息类型
	SeqID          uint64 `json:"seq_id"`
	AckID          uint64 `json:"ack_id"`
}

// ReplyMsg 服务器回复消息结构体
type ReplyMsg struct {
	Cmd            string `json:"cmd"` // "ack、text"
	ConversationID string `json:"conversation_id"`
	ClientMsgID    string `json:"client_msg_id"`
	Flag           uint8  `json:"flag"`
	Content        string `json:"content"`
	SenderID       uint64 `json:"sender_id"`
	ReceiverID     uint64 `json:"receiver_id"`
	TimeStamp      int64  `json:"time_stamp"`
	SeqID          uint64 `json:"seq_id"`
	AckID          uint64 `json:"ack_id"`
}

// AckPayload 客户端发送的确认包
type AckPayload struct {
	Cmd            string `json:"cmd"`             // "delivered" 或 "read"
	ConversationID string `json:"conversation_id"` // 会话 ID
	SeqID          int64  `json:"seq_id"`          // 确认收到的最大 SeqID
}

func (msg *SendMsg) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *ReplyMsg) Deserialize(data []byte) error {
	return json.Unmarshal(data, msg)
}

func (msg *SendMsg) GetCmd() string {
	return msg.Cmd
}
func (msg *SendMsg) GetSeqID() uint64 {
	return msg.SeqID
}
func (msg *SendMsg) GetReceiverID() uint64 {
	return msg.ReceiverID
}
func (msg *SendMsg) GetSenderID() uint64 {
	return msg.SenderID
}
func (msg *SendMsg) GetConversationID() string {
	return msg.ConversationID
}

func (msg *ReplyMsg) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}
func (msg *SendMsg) Deserialize(data []byte) error {
	return json.Unmarshal(data, msg)
}

func (msg *ReplyMsg) GetCmd() string {
	return msg.Cmd
}
func (msg *ReplyMsg) GetSeqID() uint64 {
	return msg.SeqID
}
func (msg *ReplyMsg) GetReceiverID() uint64 {
	return msg.ReceiverID
}
func (msg *ReplyMsg) GetSenderID() uint64 {
	return msg.SenderID
}
func (msg *ReplyMsg) GetConversationID() string {
	return msg.ConversationID
}

func (msg *ReplyMsg) SetContent(content string) *ReplyMsg {
	msg.Content = content
	return msg
}

func (msg *AckPayload) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}
func (msg *AckPayload) Deserialize(data []byte) error {
	return json.Unmarshal(data, msg)
}
func (msg *AckPayload) GetCmd() string {
	return msg.Cmd
}
func (msg *AckPayload) GetSeqID() uint64 {
	return 0
}
func (msg *AckPayload) GetReceiverID() uint64 {
	return 0
}
func (msg *AckPayload) GetSenderID() uint64 {
	return 0
}
func (msg *AckPayload) GetConversationID() string {
	return ""
}

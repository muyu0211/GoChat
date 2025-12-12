package websocket

/**

 */

const (
	CmdChat   = "chat"
	CmdAck    = "ack"
	CmdRevoke = "revoke"
	CmdDelete = "delete"
	CmdError  = "error"
)

// SendMsg 客户端发送给服务器的	消息结构体
type SendMsg struct {
	Cmd         string `json:"cmd"`
	ClientMsgID string `json:"client_msg_id"` // 每条消息的全局唯一ID（客户端生成），可以是UUID, TODO: 区分网络重试和业务重试()
	Flag        uint8  `json:"flag"`
	Content     string `json:"content"`
	SenderID    uint64 `json:"sender_id"`
	ReceiverID  uint64 `json:"receiver_id"`
	TimeStamp   int64  `json:"time_stamp"` // 发送消息的时间戳
	MsgType     byte   `json:"msg_type"`   // 消息类型
	SeqID       uint64 `json:"seq_id"`
	AckID       uint64 `json:"ack_id"`
}

// ReplyMsg 服务器回复消息结构体
type ReplyMsg struct {
	Cmd         string `json:"cmd"`           // "ack、text"
	ClientMsgID string `json:"client_msg_id"` // 原样返回，方便前端对应
	Flag        uint8  `json:"flag"`
	Content     string `json:"content"`
	SenderID    uint64 `json:"sender_id"`
	ReceiverID  uint64 `json:"receiver_id"` // 接收者ID
	TimeStamp   int64  `json:"time_stamp"`
	SeqID       uint64 `json:"seq_id"`
	AckID       uint64 `json:"ack_id"`
}

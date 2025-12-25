package dto

type UpdatesAck struct {
	UserID         uint64
	ConversationID string
	LastAckID      uint64
}

type ConverseMessage struct {
	ConversationID string `json:"conversation_id"`
	StartTime      uint64 `json:"start_time"` // 时间戳
	LastAckID      uint64 `json:"last_ack_id"`
}

type GetMsgHistoryReq struct {
	UserID         uint64 `json:"user_id" form:"user_id" binding:"omitempty"`
	ConversationID string `json:"conversation_id" form:"conversation_id"`
	StartTime      uint64 `json:"start_time" form:"start_time" binding:"omitempty"`
	EndTime        uint64 `json:"end_time" form:"end_time" binding:"omitempty"`
	Limit          int64  `json:"limit" form:"limit"`
	LastAckID      uint64 `json:"last_ack_id" form:"last_ack_id" binding:"omitempty"`
}

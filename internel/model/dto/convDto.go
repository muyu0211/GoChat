package dto

type UpdatesAck struct {
	UserID         uint64
	ConversationID string
	LastAckID      uint64
}

package dto

type SendCodeRequest struct {
	EmailOrPhone string `json:"email_or_phone" form:"email_or_phone"`
	Scene        string `json:"scene" form:"scene"`
}

type RegisterRequest struct {
	Nickname     string `json:"nickname" form:"nickname"`
	EmailOrPhone string `json:"email_or_phone" form:"email_or_phone" `
	Password     string `json:"password" form:"password" binding:"omitempty"`
	VerifyCode   string `json:"verify_code" form:"verify_code" binding:"omitempty"`
	State        byte   `json:"state" form:"state" binding:"omitempty"`
}

type LoginRequest struct {
	EmailOrPhone string `json:"email_or_phone" form:"email_or_phone"`
	Password     string `json:"password" form:"password" binding:"omitempty"`
	VerifyCode   string `json:"verify_code" form:"verify_code" binding:"omitempty"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Telephone string `json:"phone" binding:"omitempty"`
}

type UserSession struct {
	UserID uint64 `json:"user_id"`
	Convs  []Conversations
}

type Conversations struct {
	ConversationID string `json:"conversation_id"`
	LastSeq        uint64 `json:"last_seq"`
	UnreadCount    uint64 `json:"unread_count"`
}

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

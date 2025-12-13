package util

import (
	"github.com/google/uuid"
	"time"
)

var (
	ServerID = "node-" + uuid.New().String()
)

const (
	KeyVerifyCode          = "verify_code:"
	KeyUserLocation        = "im:user:location"
	VerificationCodeLength = 6

	CodeExpireTime         = 60 * time.Second
	CodeUserLocationExpire = 7 * 24 * time.Hour
	CodeMaxSendNum         = 2 // 验证码重发次数

	CHARSET              = "abcdefghijklmnopqrstuWXyZABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	UsernameSuffixLength = 12

	EmailTitle = "UrCloth验证码"
	EmailHost  = "smtp.qq.com"
	EmailPort  = 25
	EmailUser  = "2193442725@qq.com"
	EmailPwd   = "xihomhdsleswdjcb"

	RegisterInEmail = "email"
	RegisterInPhone = "phone"

	LoginInEmailWithCode     = "email_code"
	LoginInPhoneWithCode     = "phone_code"
	LoginInEmailWithPassword = "email_password"
	LoginInPhoneWithPassword = "phone_password"
)

const (
	ErrService = "服务错误"
)

const (
	CtxUserIDKey = "UserID"
	CtxEmailKey  = "Email"
	CtxPhoneKey  = "Phone"
	CtxRoleKey   = "Role"
)

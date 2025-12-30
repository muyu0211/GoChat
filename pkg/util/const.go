package util

import (
	"GoChat/config"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
)

var (
	ServerID string
	node     *snowflake.Node
	once     sync.Once
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

	EmailTitle = "GoChat验证码"
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

	GroupIDOffSet      uint64 = 2_000_000
	GroupDefaultMemNum        = 500
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

func Init() {
	ServerID = "node-" + fmt.Sprintf("%d", config.Cfg.BasicConfig.ServerID)
}

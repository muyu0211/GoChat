package service

import "errors"

// 定义一些业务错误，便于 Handler 层进行判断
var (
	ErrServerNotAvailable = errors.New("服务器繁忙，请稍后再试")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrInvalidCredentials = errors.New("邮箱/手机号格式错误")
	ErrUserDisabled       = errors.New("用户已封禁")
	ErrCodeError          = errors.New("验证码错误")
	ErrPWError            = errors.New("密码错误")
	ErrUserAlreadyExist   = errors.New("用户已存在")

	ErrUserOffline = errors.New("用户已下线")

	ErrMarshalJSON = errors.New("JSON序列化错误")
	ErrPushMsg     = errors.New("消息推送错误")
)

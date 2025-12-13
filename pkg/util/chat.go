package util

import "time"

const (
	// PubSubChannel Redis发布订阅频道（统一配置，避免硬编码）
	PubSubChannel  = "im:push:cross_server"
	RedisDupKey    = "im:dup_key"
	RedisDupExpire = 5 * time.Minute
	RedisSeqKey    = "im:seq"
	// RedisPublishTimeout Redis发布操作超时时间（避免阻塞）
	RedisPublishTimeout = 3 * time.Second
	// RedisPublishRetryTimes 发布失败重试次数（幂等场景下适用）
	RedisPublishRetryTimes = 3
	// RedisPublishRetryInterval 重试间隔
	RedisPublishRetryInterval = 100 * time.Millisecond

	PushLocalTimeout = 3 * time.Second
)

// 定义消息类型枚举 (避免魔法数字)
const (
	_            = iota
	MsgTypeText  // 文本
	MsgTypeImage // 图片 (Content 存 URL)
	MsgTypeAudio // 语音 (Content 存 URL)
	MsgTypeVideo // 视频 (Content 存 URL)
)

// 定义消息状态枚举
const (
	MsgStatusRead    = iota // 已读
	MsgStatusRevoked        // 撤回
	MsgStatusDeleted        // 删除
)

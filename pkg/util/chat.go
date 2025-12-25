package util

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
	MsgStatusRead    = iota // 正常
	MsgStatusRevoked        // 撤回
	MsgStatusDeleted        // 删除
)

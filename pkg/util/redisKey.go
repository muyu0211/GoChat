package util

import (
	"fmt"
	"strconv"
	"time"
)

const (
	PubSubChannel   = "im:push:cross_server"
	RedisDupKey     = "im:dup_key"
	RedisBoxKey     = "im:offline_msg_box"
	RedisConvKey    = "im:conversation"
	RedisSeqKey     = "im:seq"
	RedisSeqLockKey = "im:seq:lock"

	RedisDupExpire            = 1 * time.Second
	RedisSeqExpire            = 90 * 24 * time.Hour
	RedisOfflineExpire        = 7 * 24 * time.Hour
	RedisPublishTimeout       = 3 * time.Second
	RedisPublishRetryTimes    = 3
	RedisPublishRetryInterval = 100 * time.Millisecond

	PushLocalTimeout = 3 * time.Second

	RedisZAddTimeout = 500 * time.Millisecond
)

func GetRedisPubSubChannel() string {
	return PubSubChannel
}

func GetRedisSeqKey(conversationID string) string {
	return fmt.Sprintf("%s:%s", RedisSeqKey, conversationID)
}

func GetRedisSeqLockKey(conversation string) string {
	return fmt.Sprintf("%s:%s", RedisSeqLockKey, conversation)
}

func GetRedisConvKey(userID uint64) string {
	return fmt.Sprintf("%s:%s", RedisConvKey, strconv.FormatUint(userID, 10))
}

func GetRedisBoxKey(userID uint64, conversationID string) string {
	return fmt.Sprintf("%s:%s:%s", RedisBoxKey, strconv.FormatUint(userID, 10), conversationID)
}
func GetRedisDupKey(conversationID, clientMsgID string) string {
	return fmt.Sprintf("%s:%s:%s", RedisDupKey, conversationID, clientMsgID)
}

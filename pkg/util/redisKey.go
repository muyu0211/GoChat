package util

import (
	"fmt"
	"strconv"
	"time"
)

const (
	PubSubChannel         = "im:push:cross_server"
	RedisDupKey           = "im:dup_key"
	RedisBoxKey           = "im:offline_msg_box"
	RedisConvKey          = "im:conversation"
	RedisChatSeqKey       = "im:chat:seq"
	RedisSeqLockKey       = "im:seq:lock" // 初始化会话 SeqID 的分布式锁
	RedisGroupIDKey       = "im:group:id"
	RedisGroupIDOffsetKey = "im:group:id:offset"
	RedisKafkaGroupDupKey = "im:kafka:group:dup" // Kafka 群消息去重

	RedisDupExpire            = 1 * time.Second
	RedisKafkaDupExpire       = 24 * time.Hour // Kafka 消息去重过期时间
	RedisSeqExpire            = 90 * 24 * time.Hour
	RedisOfflineExpire        = 7 * 24 * time.Hour
	RedisGroupIDExpire        = 30 * 24 * time.Hour
	RedisPublishTimeout       = 3 * time.Second
	RedisPublishRetryTimes    = 3
	RedisPublishRetryInterval = 100 * time.Millisecond

	PushLocalTimeout = 3 * time.Second

	RedisZAddTimeout = 500 * time.Millisecond
)

func GetRedisPubSubChannel() string {
	return PubSubChannel
}

func GetRedisChatSeqKey(conversationID string) string {
	return fmt.Sprintf("%s:%s", RedisChatSeqKey, conversationID)
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

func GetRedisGroupIDKey[T string | uint64](groupID T) string {
	switch v := any(groupID).(type) {
	case string:
		return fmt.Sprintf("%s:%s", RedisGroupIDKey, v)
	case uint64:
		return fmt.Sprintf("%s:%d", RedisGroupIDKey, v)
	default:
		return fmt.Sprintf("%s:%v", RedisGroupIDKey, v)
	}
}

func GetRedisGroupIDOffsetKey() string {
	return RedisGroupIDOffsetKey
}

func GetRedisKafkaGroupDupKey(groupID uint64, seqID uint64) string {
	return fmt.Sprintf("%s:%d:%d", RedisKafkaGroupDupKey, groupID, seqID)
}

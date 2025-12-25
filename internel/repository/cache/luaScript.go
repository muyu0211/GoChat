package cache

// DedupAndSeqLua 返回值定义：0 -重复消息，-1 -seqKey不存在需要初始化，>0 -正常返回
var DedupAndSeqLua = `
local dupKey = KEYS[1]
local seqKey = KEYS[2]
local dupExpire = tonumber(ARGV[1])

-- 1. 去重：先占位
local ok = redis.call('SET', dupKey, 1, 'NX', 'EX', dupExpire)
if not ok then
    return 0  -- 重复消息
end

-- 2. seqKey 不存在 → 需要初始化
if redis.call('EXISTS', seqKey) == 0 then
    return -1
end

-- 3. 正常路径：递增并返回
return redis.call('INCR', seqKey)
`

-- token_bucket.lua
--
-- KEYS[1] = bucket key
--
-- ARGV[1] = capacity
-- ARGV[2] = refill rate (tokens per second)
-- ARGV[3] = current time (milliseconds)
-- ARGV[4] = bucket TTL (milliseconds)
--
-- Returns:
--   {1, 0}                  -> allowed
--   {0, retry_after_ms}     -> rejected

local key = KEYS[1]

local capacity      = tonumber(ARGV[1])
local refill_rate   = tonumber(ARGV[2])
local now_ms        = tonumber(ARGV[3])
local ttl_ms        = tonumber(ARGV[4])

if not capacity or capacity <= 0 then
    return redis.error_reply("capacity must be greater than 0")
end

if not refill_rate or refill_rate <= 0 then
    return redis.error_reply("refill rate must be greater than 0")
end

if not now_ms then
    return redis.error_reply("current time is required")
end

if not ttl_ms or ttl_ms <= 0 then
    return redis.error_reply("TTL must be greater than 0")
end

local bucket = redis.call(
    "HMGET",
    key,
    "tokens",
    "last_refill_ms"
)

local tokens        = tonumber(bucket[1])
local last_refill_ms = tonumber(bucket[2])

-- First request for this client:
-- start with a full bucket.
if tokens == nil or last_refill_ms == nil then
    tokens        = capacity
    last_refill_ms = now_ms
end

-- Refill based on elapsed time.
local elapsed_ms = math.max(0, now_ms - last_refill_ms)

tokens = math.min(
    capacity,
    tokens + (elapsed_ms / 1000) * refill_rate
)

local allowed        = 0
local retry_after_ms = 0

if tokens >= 1 then
    tokens  = tokens - 1
    allowed = 1
else
    retry_after_ms = math.ceil(
        ((1 - tokens) / refill_rate) * 1000
    )
end

-- Save the updated bucket state.
redis.call(
    "HSET",
    key,
    "tokens",        tokens,
    "last_refill_ms", now_ms
)

-- Remove idle client buckets eventually.
redis.call("PEXPIRE", key, ttl_ms)

return {
    allowed,
    retry_after_ms
}

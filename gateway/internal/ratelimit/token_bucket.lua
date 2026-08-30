-- token_bucket.lua
--
-- KEYS[1] = bucket key
--
-- ARGV[1] = capacity
-- ARGV[2] = refill rate (tokens per second)
-- ARGV[3] = current time (milliseconds)
-- ARGV[4] = bucket TTL (milliseconds)
-- ARGV[5] = gateway ID (optional, for proof counters)
--
-- Returns:
--   {1, 0}                  -> allowed
--   {0, retry_after_ms}     -> rejected

local key = KEYS[1]

local capacity      = tonumber(ARGV[1])
local refill_rate   = tonumber(ARGV[2])
local now_ms        = tonumber(ARGV[3])
local ttl_ms        = tonumber(ARGV[4])
local gateway_id    = ARGV[5]

if not capacity or capacity <= 0 then
    return redis.error_reply("capacity must be greater than 0")
end

if not refill_rate or refill_rate < 0 then
    return redis.error_reply("refill rate cannot be negative")
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

local tokens         = tonumber(bucket[1])
local last_refill_ms = tonumber(bucket[2])

-- First request for this client:
-- start with a full bucket.
if tokens == nil or last_refill_ms == nil then
    tokens         = capacity
    last_refill_ms = now_ms
end

-- Refill based on elapsed time.
if refill_rate > 0 then
    local elapsed_ms = math.max(0, now_ms - last_refill_ms)

    tokens = math.min(
        capacity,
        tokens + (elapsed_ms / 1000) * refill_rate
    )
end

local allowed        = 0
local retry_after_ms = 0

if tokens >= 1 then
    tokens  = tokens - 1
    allowed = 1
else
    if refill_rate > 0 then
        retry_after_ms = math.ceil(
            ((1 - tokens) / refill_rate) * 1000
        )
    else
        -- No refill: tokens will never come back.
        retry_after_ms = -1
    end
end

-- Save the updated bucket state.
redis.call(
    "HSET",
    key,
    "tokens",         tokens,
    "last_refill_ms", now_ms
)

-- Remove idle client buckets eventually.
redis.call("PEXPIRE", key, ttl_ms)

-- Proof counters: increment atomically inside the same script
-- so the count is guaranteed to match the actual admission decision.
-- NOTE: these keys are outside KEYS[], which is fine for single-instance
-- Redis but would need adjustment for Redis Cluster.
if allowed == 1 and gateway_id and gateway_id ~= "" then
    redis.call("INCR", "proof:allowed")
    redis.call("INCR", "proof:allowed:" .. gateway_id)
end

return {
    allowed,
    retry_after_ms
}

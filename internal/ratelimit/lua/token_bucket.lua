local key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = burst
    last_refill = now
end

local elapsed = now - last_refill
local refill = elapsed * rate
tokens = math.min(burst, tokens + refill)

if tokens < 1 then
    return {0, tokens, burst}
end

tokens = tokens - 1

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
redis.call('EXPIRE', key, math.ceil(burst / rate) * 2)

return {1, tokens, burst}
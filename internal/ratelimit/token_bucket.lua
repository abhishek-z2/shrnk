local key = KEYS[1]

local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local refill_rate = tonumber(ARGV[3])

local tokens = tonumber(redis.call("HGET", key, "tokens"))
local last_refill = tonumber(redis.call("HGET", key, "last_refill"))

if not tokens or not last_refill then
	tokens = capacity
	last_refill = now
else
	local elapsed = math.max(0, now - last_refill)

	tokens = tokens + (elapsed * refill_rate / 1000)

	if tokens > capacity then
		tokens = capacity
	end
end

if tokens < 1 then
	redis.call("HSET", key, "tokens", tokens, "last_refill", now)

	return 0
end

tokens = tokens - 1

redis.call("HSET", key, "tokens", tokens, "last_refill", now)

return 1

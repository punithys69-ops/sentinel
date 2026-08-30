# Phase 2.3 Tasks

- [x] Lua script exists at limiter/token_bucket.lua
- [x] RedisLimiter struct in redis_limiter.go
- [x] go-redis/v9 in go.mod
- [/] Add RedisRateLimitMiddleware to middleware.go
- [ ] go build ./... passes
- [ ] Start Redis container
- [ ] redis-cli ping → PONG
- [ ] SCRIPT LOAD → SHA
- [ ] 5-token burst test
- [ ] 6th request rejection test
- [ ] Refill test
- [ ] HGETALL inspection

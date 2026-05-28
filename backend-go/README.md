# Go Backend

## Redis usage

The Go backend uses Redis for short-lived cache, update coordination, and async task status storage. MongoDB remains the source of truth for fund data.

Redis connection configuration:

- `REDIS_URL` is preferred when it is set. This supports managed Redis URLs such as `redis://` or `rediss://`.
- `REDIS_ADDR` is used when `REDIS_URL` is empty.
- If both variables are empty, the backend defaults to `127.0.0.1:6379`.

### Fund detail cache

- Key: `fund:detail:{code}`
- Value: fund detail JSON
- TTL: 60 seconds
- `GET /api/fund/{code}` uses the Cache Aside pattern.
- On cache hit, the API returns the cached JSON with `X-Cache: HIT`.
- On cache miss, the API reads the fund detail from MongoDB, writes the JSON to Redis, and returns `X-Cache: MISS`.
- After `/api/update` or `/api/update/async` successfully updates funds, the backend deletes `fund:detail:{code}` for each code in `updated_codes`.

### Fund update lock

- Key: `lock:fundtracking:update`
- The backend acquires the lock with `SETNX` and a TTL.
- The lock value is a random token.
- The backend releases the lock with a Lua script that checks the token before calling `DEL`.
- If the lock cannot be acquired, `/api/update` and `/api/update/async` return `409 update_locked`.
- The lock prevents synchronous and asynchronous update requests from writing fund data at the same time.

### Async update task status

- Key: `fund:update:task:{taskID}`
- Value: serialized `updateTask` JSON
- TTL: 1 hour
- Status values: `pending`, `running`, `success`, `failed`
- `/api/update/async` writes a task record to Redis after creating the task.
- The background goroutine updates the task status to `running`, then to `success` or `failed`.
- `GET /api/update/tasks/{taskID}` reads the task status from Redis.
- If the key does not exist or has expired, the API returns `404 task not found`.
- Redis is used for async task status because it fits short-lived runtime data better than a Go map with a mutex. It also keeps task status available across requests, makes recent status queryable after a process restart, and allows multiple backend instances to share the same task state.

# Go URL Shortener

A URL shortener built with Go and the standard-library `net/http` package. It supports persistent links, custom aliases, expiry, Redis caching, and redirect statistics.

PostgreSQL stores URL records and click counts. Redis caches URL lookups. An in-memory backend supports local use and tests.

## Features

- Create short links with random codes or custom aliases.
- Redirect active links with `302 Found`.
- Set optional expiry deadlines and return `410 Gone` after expiry.
- Read URL metadata and daily click statistics.
- Protect deletion and statistics with a management API key.
- Cache lookups with finite, expiry-aware TTLs.
- Apply request IDs, creation rate limits, and request deadlines.
- Run with Docker Compose and verify changes through GitHub Actions.

## Requirements

For the container setup:

- Docker Desktop with the Linux-container engine running.
- Docker Compose.
- PowerShell for the commands below.

For local Go development:

- Go 1.26.6, as specified in `go.mod`.
- PostgreSQL and Redis for integration tests.

The repository uses pgx v5.11.0 and go-redis v9.23.0. Compose uses PostgreSQL 18.6, Redis 8, and migrate v4.20.1.

## Quick start

Run these commands from the repository root. Port 8080 must be available.

### 1. Configure the stack

Enter development passwords and a nonempty management key. These values remain in the current PowerShell process.

~~~powershell
$adminInput = Read-Host 'PostgreSQL admin password' -AsSecureString
$env:POSTGRES_ADMIN_PASSWORD = [System.Net.NetworkCredential]::new('', $adminInput).Password

$appInput = Read-Host 'Application database password' -AsSecureString
$env:APP_DATABASE_PASSWORD = [System.Net.NetworkCredential]::new('', $appInput).Password

$keyInput = Read-Host 'Management API key' -AsSecureString
$env:MANAGEMENT_API_KEY = [System.Net.NetworkCredential]::new('', $keyInput).Password

$encodedPassword = [Uri]::EscapeDataString($env:APP_DATABASE_PASSWORD)
$env:COMPOSE_DATABASE_URL = "postgres://url_shortener_dev:${encodedPassword}@postgres:5432/url_shortener?sslmode=disable&pool_max_conns=10"
$env:PUBLIC_BASE_URL = 'http://127.0.0.1:8080'
~~~

Do not commit credentials. The Go application reads environment variables. It does not load a `.env` file automatically.

### 2. Build and start

~~~powershell
docker compose config --quiet
docker compose up --detach --build app
docker compose ps --all
~~~

Compose waits for healthy PostgreSQL and Redis services. It applies pending migrations before it starts the application.

The first build can take several minutes. The migration service should exit with code 0.

### 3. Check the service

~~~powershell
curl.exe --fail --show-error -i http://127.0.0.1:8080/healthz
curl.exe --fail --show-error -i http://127.0.0.1:8080/readyz
~~~

Both endpoints should return `200 OK`.

| Service | Address on Windows | Address inside Compose |
| --- | --- | --- |
| Application | `127.0.0.1:8080` | `app:8080` |
| PostgreSQL | `127.0.0.1:5433` | `postgres:5432` |
| Redis | `127.0.0.1:6380` | `redis:6379` |

Use service names inside containers. Use published ports for host tools.

## API examples

These examples use the stack from Quick start.

### Create a short link

~~~powershell
$response = Invoke-WebRequest `
    -Uri 'http://127.0.0.1:8080/api/v1/urls' `
    -Method Post `
    -ContentType 'application/json' `
    -Body '{"url":"https://example.com/articles/go"}'

$link = $response.Content | ConvertFrom-Json
$link
~~~

The response status is `201 Created`. Example response:

~~~json
{
  "code": "abc12345",
  "url": "https://example.com/articles/go",
  "short_url": "http://127.0.0.1:8080/abc12345"
}
~~~

Create a link with a custom alias and expiry:

~~~powershell
$payload = @{
    url = 'https://example.com/guide'
    alias = 'go-guide'
    expires_at = [DateTimeOffset]::UtcNow.AddHours(1).ToString('o')
} | ConvertTo-Json -Compress

Invoke-WebRequest `
    -Uri 'http://127.0.0.1:8080/api/v1/urls' `
    -Method Post `
    -ContentType 'application/json' `
    -Body $payload
~~~

An occupied alias returns `409 Conflict`. Create a new future deadline when you repeat an expiring request.

### Read metadata and follow the redirect

~~~powershell
$metadataURL = "http://127.0.0.1:8080/api/v1/urls/$($link.code)"
curl.exe -i $metadataURL
curl.exe -i $link.short_url
~~~

Metadata returns `200 OK` with `code`, `url`, and optional `expires_at`.

A GET redirect returns `302 Found` with the destination in `Location`. The command above does not follow that destination.

Use HEAD to inspect redirect headers without recording a click:

~~~powershell
curl.exe --head $link.short_url
~~~

### Read statistics

~~~powershell
$managementHeaders = @{
    Authorization = "Bearer $env:MANAGEMENT_API_KEY"
}
$statsURL = "$metadataURL/stats"

Invoke-RestMethod -Uri $statsURL -Headers $managementHeaders
~~~

Example response after one GET redirect:

~~~json
{
  "code": "abc12345",
  "total_clicks": 1,
  "daily": [
    {
      "date": "2026-11-01",
      "clicks": 1
    }
  ]
}
~~~

Statistics use UTC dates. A link with no clicks returns `"daily":[]`.

### Delete a link

~~~powershell
$result = Invoke-WebRequest `
    -Uri $metadataURL `
    -Method Delete `
    -Headers $managementHeaders

$result.StatusCode
~~~

Successful deletion returns `204 No Content`. Later metadata, redirect, and statistics requests return `404 Not Found`.

Deletion removes the associated counters. A new record can reuse the code, but it receives a new ID and starts with zero clicks.

## API reference

| Method | Route | Purpose | Success | Authentication |
| --- | --- | --- | --- | --- |
| POST | `/api/v1/urls` | Create a link | 201 | Public |
| GET | `/api/v1/urls/{code}` | Read active-link metadata | 200 | Public |
| GET | `/{code}` | Record a click and redirect | 302 | Public |
| HEAD | `/{code}` | Return redirect headers without a click | 302 | Public |
| GET | `/api/v1/urls/{code}/stats` | Read total and daily counts | 200 | Bearer key |
| DELETE | `/api/v1/urls/{code}` | Delete a link and its counters | 204 | Bearer key |
| GET | `/healthz` | Check that the HTTP process responds | 200 | Public |
| GET | `/readyz` | Check primary-storage connectivity | 200 | Public |
| GET | `/` | Return a basic greeting | 200 | Public |

Management routes are registered only when `MANAGEMENT_API_KEY` is nonempty. Missing or invalid credentials return `401` when those routes are enabled.

### Creation rules

- `url` must be an absolute HTTP or HTTPS URL with a hostname.
- The body limit is 4 KiB, including trailing whitespace.
- The body must contain one JSON value. Unknown fields are rejected.
- Omit `alias`, or set it to `null`, to generate a random code.
- Custom aliases accept 3–32 ASCII letters, digits, hyphens, or underscores.
- Alias case is preserved. Reserved names are `healthz`, `readyz`, and `api`, compared without case.
- An explicitly empty alias is invalid.
- `expires_at` must be an RFC3339 timestamp with a timezone and a future deadline.
- Omitted or null expiry creates a permanent link.
- Expiry is normalized to UTC and microsecond precision before storage.

At the exact expiry deadline, the link is expired. Expiry prevents redirects and metadata access. It does not remove the record or release its code.

Statistics and deletion remain available for expired records.

### Failure responses

| Status | Condition |
| --- | --- |
| 400 | Invalid JSON, URL, alias, or expiry |
| 401 | Missing or invalid management credentials |
| 404 | Record missing or path not registered |
| 409 | Custom alias already occupied |
| 410 | Link expired |
| 413 | Creation body exceeds the size limit |
| 429 | Creation rate limit exceeded |
| 500 | Unexpected storage or internal failure |
| 503 | Allocation attempts exhausted, reported request deadline exceeded, or readiness check failed |

Successful data responses use JSON. Error responses use plain text. Internal error details are logged instead of returned to clients.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `127.0.0.1:8080` | HTTP listening address |
| `PUBLIC_BASE_URL` | `http://127.0.0.1:8080` | Public origin used in returned short links |
| `STORAGE_BACKEND` | `memory` | Select `memory` or `postgres` |
| `DATABASE_URL` | Unset | PostgreSQL connection URL |
| `REDIS_URL` | Unset | Enable Redis lookup caching in PostgreSQL mode |
| `MANAGEMENT_API_KEY` | Unset | Enable protected deletion and statistics |
| `TEST_DATABASE_URL` | Unset | Enable PostgreSQL integration tests |
| `TEST_REDIS_URL` | Unset | Enable Redis integration tests |

The Docker image sets `HTTP_ADDR=0.0.0.0:8080`. Compose supplies PostgreSQL and Redis connection URLs.

`PUBLIC_BASE_URL` must use HTTP or HTTPS and include a hostname. Credentials, queries, fragments, and path prefixes are rejected. A trailing slash is removed.

Set the public origin separately from the listen address when a reverse proxy provides HTTPS. Creation does not check public-origin reachability.

A whitespace-only management key fails startup. Redis configuration requires `STORAGE_BACKEND=postgres`.

Compose also requires `POSTGRES_ADMIN_PASSWORD`, `APP_DATABASE_PASSWORD`, and `COMPOSE_DATABASE_URL`. These variables configure the stack rather than the Go process directly.

### Database pool settings

Pool settings can be included in `DATABASE_URL` or `COMPOSE_DATABASE_URL`:

~~~text
postgres://USER:ENCODED_PASSWORD@HOST:PORT/DATABASE?sslmode=disable&pool_max_conns=10
~~~

The quick start uses ten connections per application process. This is a development example, not a measured optimum.

Budget connections across all application instances. Leave capacity for migrations, administration, and other database clients.

## Architecture

~~~mermaid
flowchart LR
    Client --> HTTP["Go HTTP server"]
    HTTP --> Service["Creation service"]
    Service --> Primary["Primary storage"]
    HTTP --> Cache["Cache-aside URL lookup"]
    Cache --> Redis["Optional Redis cache"]
    Cache --> Primary
    HTTP --> Analytics["Click recording and statistics"]
    Analytics --> Primary
~~~

Handlers translate HTTP requests and responses. The creation service validates input and allocates codes. Narrow interfaces separate storage, deletion, and analytics.

### Storage and identity

PostgreSQL enforces code uniqueness with a database constraint. Random-code creation retries collisions up to five times. Custom aliases use one insertion attempt.

Records have stable IDs. Analytics use the ID rather than the reusable short code. PostgreSQL deletes daily counters through a cascading foreign key.

The memory backend protects records, IDs, and counters with a shared lock. Memory data is lost when the process stops.

### Cache behavior

Creation writes to primary storage only. A lookup checks Redis first, then reads PostgreSQL on a miss and caches the returned record.

Cache TTL is the smaller of five minutes and the remaining link lifetime. Permanent links still have a finite TTL. Hits do not refresh TTL.

A Redis read or population failure does not discard a successful primary lookup. Cache operations use short child contexts.

Deletion invalidates Redis before deleting from PostgreSQL. If invalidation fails, primary deletion stops. This prevents successful deletion from leaving a known cached record behind.

Each process run uses a new Redis namespace. This avoids reading entries from a previous run. It does not coordinate invalidation between multiple active application instances.

### Click statistics

GET redirects synchronously record a click before sending `302`. HEAD, metadata, and statistics requests do not add clicks.

A click represents a recorded redirect request. It does not confirm that the client reached the destination. Retries can add multiple clicks.

Counters are grouped by UTC date. PostgreSQL increments daily counts atomically. Statistics remain available after expiry and are removed on deletion.

Click recording does not use Redis or a background queue. A warm cache cannot make counted GET redirects succeed during a PostgreSQL outage.

### Dependency failures

| Condition | Metadata and HEAD | GET redirect | Statistics | Deletion |
| --- | --- | --- | --- | --- |
| Redis unavailable; PostgreSQL available | Primary fallback | Records click and redirects | Available | Fails while invalidation is unavailable |
| PostgreSQL unavailable; active record cached | Can use cached record | Fails during click recording | Fails | Primary deletion fails |
| PostgreSQL unavailable; cache miss | Fails | Fails | Fails | Fails |

`/healthz` checks the process only. In PostgreSQL mode, `/readyz` calls `pool.Ping`. It does not verify the schema or Redis.

## Local development

### Run without containers

Use memory storage for a minimal local server:

~~~powershell
$env:STORAGE_BACKEND = 'memory'
$env:REDIS_URL = ''
$env:HTTP_ADDR = '127.0.0.1:8080'
$env:PUBLIC_BASE_URL = 'http://127.0.0.1:8080'
go run ./cmd/server
~~~

For PostgreSQL mode, supply a reachable `DATABASE_URL` and apply both migrations first. Never use a PostgreSQL superuser as the application login.

### Migrations

The schema has two migration versions:

| Version | Change |
| --- | --- |
| 1 | URL records, identity column, code uniqueness, and optional expiry |
| 2 | Daily click counters, composite primary key, and cascading deletion |

Compose applies pending migrations automatically before application startup. Run them separately with:

~~~powershell
docker compose run --rm migrate
~~~

Inspect the current version:

~~~powershell
docker compose exec postgres psql `
    -h 127.0.0.1 -U url_shortener_dev -d url_shortener -W `
    -c 'SELECT version, dirty FROM schema_migrations;'
~~~

Expect version `2` and `dirty = f`.

The initialization script creates the application role only for a new database volume. Changing its password variable does not rotate an existing role password.

A down migration can delete stored records or counters. Reapplying the up migration does not restore that data.

## Tests

Format code and run the standard checks:

~~~powershell
go fmt ./cmd/server
go vet ./...
go test -race -count=1 -v ./...
~~~

Integration tests skip when their test URL is unset. Configure host access to the Compose services in the quick-start terminal:

~~~powershell
$env:TEST_DATABASE_URL = "postgres://url_shortener_dev:${encodedPassword}@127.0.0.1:5433/url_shortener?sslmode=disable&pool_max_conns=10"
$env:TEST_REDIS_URL = 'redis://127.0.0.1:6380/0'
go test -race -count=1 -v ./...
~~~

Tests cover validation, strict decoding, collisions, expiry, authentication, cancellation, concurrency, cache failure, deletion, and analytics.

Run the pool-limit check separately:

~~~powershell
go test -race -count=1 -v `
    -run '^TestPostgresPoolLimitIntegration$' ./cmd/server
~~~

This check holds two pool connections. A third acquisition must reach its deadline. A released connection must then support another query.

### Continuous integration

GitHub Actions checks formatting, vet, and race-enabled unit and integration tests. It starts real PostgreSQL and Redis services and applies migrations.

CI also runs the cold/warm benchmark and builds the Docker image. Container smoke checks cover:

- Health and readiness.
- Creation, metadata, and redirect destinations.
- Protected statistics and deletion.
- HEAD behavior without click increments.
- Persistence after application restart.
- Missing records after deletion.
- Clean container shutdown.

CI uses disposable fixture credentials. It stops services after success or failure. Benchmark results appear in the job log.

## Performance

### Run the benchmarks

Set both integration-test URLs before service benchmarks.

~~~powershell
go test ./cmd/server -run '^$' `
    -bench '^BenchmarkRedirectHandlerMemory$' -benchmem -count=5

go test ./cmd/server -run '^$' `
    -bench '^BenchmarkRedirectHandlerMemoryParallel$' `
    -benchmem '-cpu=1,2,4,8,16' -count=3

go test ./cmd/server -run '^$' `
    -bench '^(BenchmarkPostgresLookup|BenchmarkRedisWarmLookup)$' `
    -benchmem '-cpu=16' -count=5

go test ./cmd/server -run '^$' `
    -bench '^BenchmarkCachedStoreLookup$' `
    -benchmem '-benchtime=100x' -count=5
~~~

The cache-aside benchmark uses established connections for both paths:

- `cold` removes and verifies the missing fixture key before each timed lookup. The lookup includes Redis, PostgreSQL, and cache population.
- `warm` starts with a cached fixture. Any primary lookup fails the benchmark.

Reset and verification requests are outside timing. Cold results must report `1 primary/op`. Warm results must report `0 primary/op`.

These benchmarks exclude HTTP and click writes. A cold key does not imply a cold process or PostgreSQL buffer cache.

### Recorded baseline

The local benchmark host used Windows, Go 1.26.6, and an AMD Ryzen 7 5800H.

| Benchmark | Runs | Median time per operation | Bytes per operation | Allocations per operation |
| --- | --- | --- | --- | --- |
| Memory redirect | 5 | 2.838 microseconds | 1264–1265 | 14 |
| PostgreSQL lookup | 5 | 253.848 microseconds | 1652–1653 | 24 |
| Warm Redis adapter lookup | 5 | 1.159125 milliseconds | 1216 | 18 |

The memory redirect includes an in-memory click increment and a new response recorder. It excludes the network, router, and middleware.

PostgreSQL ran natively on Windows. Redis ran through Docker Desktop. Their different connection paths limit the adapter comparison.

A four-processor mutex profile attributed 91.29 percent of contention delay to the memory `recordClick` stack. This is accumulated waiting time, not CPU time.

Parallel benchmark time per operation describes aggregate throughput. It does not measure individual request latency.

### Run the HTTP load test

Create a permanent link with the API example. Prepare its statistics URL and warm the metadata cache:

~~~powershell
$loadDirectory = (Resolve-Path './load').Path
$metadataURL = "http://127.0.0.1:8080/api/v1/urls/$($link.code)"
$managementHeaders = @{ Authorization = "Bearer $env:MANAGEMENT_API_KEY" }
$statsURL = "$metadataURL/stats"
curl.exe --fail --show-error -i $metadataURL
$beforeCount = (Invoke-RestMethod -Uri $statsURL -Headers $managementHeaders).total_clicks

docker run --rm `
    --network url-shortener-dev_default `
    --mount "type=bind,source=$loadDirectory,target=/scripts,readonly" `
    --env BASE_URL=http://app:8080 `
    --env "SHORT_CODE=$($link.code)" `
    --env "EXPECTED_DESTINATION=$($link.url)" `
    grafana/k6:1.4.0 run /scripts/redirect.js

$afterCount = (Invoke-RestMethod -Uri $statsURL -Headers $managementHeaders).total_clicks
$afterCount - $beforeCount
~~~

Use the default Compose project name for this network command. Adjust the network name if `COMPOSE_PROJECT_NAME` is set.

The script uses five virtual users for twenty seconds. Each iteration sends one GET and waits 100 milliseconds. It does not follow redirects.

Every request must return `302` and the correct destination. The click increase should match the successful GET count.

To compare with caching disabled, recreate the app with the override:

~~~powershell
docker compose -f compose.yaml -f compose.no-cache.yaml up --detach --no-deps app
curl.exe --fail --show-error -i http://127.0.0.1:8080/readyz
~~~

Repeat the warm-up, count snapshot, and load command. Keep the link and workload unchanged.

Stopping Redis measures outage fallback, not normal operation with caching disabled. Restore the normal configuration after testing:

~~~powershell
docker compose -f compose.yaml up --detach --no-deps app
~~~

The first reported Compose comparison produced these results:

| Cache | GET requests | Median HTTP duration | p95 | p99 | Failed requests | Click increase |
| --- | --- | --- | --- | --- | --- | --- |
| Enabled | 955 | 4.01 ms | 6.76 ms | 9.32 ms | 0 | 955 |
| Disabled | 955 | 3.97 ms | 6.27 ms | 8.31 ms | 0 | 955 |

Both runs passed all 1910 checks. Each GET still required a PostgreSQL click write.

This paced workload did not measure maximum capacity. One comparison does not establish a consistent cache benefit.

### Verification status

The existing deployment smoke checks passed in GitHub Actions. Local vet and race-enabled tests passed after the new benchmark and pool test were added.

The new real-service pool check and cache-aside measurements still need a service-enabled run. No results are claimed for skipped checks.

## Operations

### Logs and request IDs

The server returns `X-Request-ID`. The same ID appears in request logs with the method, route pattern, and duration.

Startup, access, and shutdown logs use JSON. Some internal diagnostics still use standard text logging.

~~~powershell
docker compose logs --follow app
~~~

### Timeouts and rate limiting

| Limit | Value |
| --- | --- |
| Request headers | 5 seconds |
| Complete request read | 10 seconds |
| Response write | 15 seconds |
| Idle connection | 60 seconds |
| Request context | 5 seconds |
| Readiness check | 1 second |
| Cache operation context | 200 milliseconds |
| Graceful HTTP shutdown | 5 seconds |
| Compose stop grace period | 10 seconds |

Context deadlines are cooperative. A write timeout does not cancel a database query automatically. Storage operations receive the request context.

Creation allows ten attempts per minute in one shared process window. Invalid requests also consume attempts. Excess attempts return `429`.

This limit is not per user or distributed across instances.

### Shutdown and data retention

Use Ctrl+C for the local server. Containers handle SIGTERM.

~~~powershell
docker compose stop --timeout 10 app
docker compose logs app
docker compose down --remove-orphans
~~~

Normal shutdown logs `Shutdown requested` and `Server stopped cleanly`. The process closes clients after HTTP shutdown.

The named PostgreSQL volume remains after `docker compose down`. Do not add `--volumes` unless you intend to delete the database.

Redis persistence is disabled. Cache loss does not remove URL records or statistics.

## Security and limitations

- Management uses one shared bearer key. Per-user ownership and automatic key rotation are not implemented.
- Creation and redirects are public. Deploy HTTPS through a reverse proxy for external access.
- URL validation permits localhost and private-network destinations. The server sends redirects; it does not fetch or verify destinations.
- Cache invalidation coordinates one process only. Multiple active instances and external database writes can leave stale entries until TTL expiry.
- Concurrent cache misses are not coalesced.
- Expired links remain stored until explicit deletion.
- Statistics have no date filters, pagination, or retention policy.
- Synchronous click writes add PostgreSQL work to every GET redirect.
- Counters do not provide unique-visitor or exactly-once metrics.
- Readiness checks connectivity, not schema correctness or Redis availability.
- Request logs do not include response status or byte count.
- No production latency or capacity guarantee has been established.

## Repository layout

~~~text
cmd/server/                 HTTP server, service, storage, middleware, tests, benchmarks
migrations/                 Versioned PostgreSQL schema changes
docker/postgres/            First-start application-role setup
load/redirect.js            k6 redirect workload
.github/workflows/ci.yaml   Tests, benchmarks, image build, and container smoke checks
Dockerfile                  Static Linux build and non-root runtime
compose.yaml                Local application, PostgreSQL, migrations, and Redis
compose.no-cache.yaml       Application override that disables Redis caching
~~~

# Go URL Shortener

This project teaches Go and backend development. It uses the standard-library `net/http` package.

The server creates short links with optional aliases and expiry times. It redirects active links and returns metadata. A management API key protects deletion.

The server uses memory storage or PostgreSQL. PostgreSQL mode can also use Redis as a cache. PostgreSQL records remain after server restarts. Memory records do not.

The server limits creation attempts and assigns request IDs. It also provides structured request logs, health checks, readiness checks, timeouts, and graceful shutdown.

The developer completed implementation and functional checks for Modules 0 through 6. These modules cover HTTP, memory storage, interfaces, PostgreSQL, URL lifecycle, Redis, and operational safeguards.

The developer typed the Module 6 code and confirmed an understanding of its functions. The full test suite passed with the race detector and both integration services enabled.

Module 7 covers analytics. The optional update endpoint is not implemented. Cache behavior has passed functional checks. No performance improvement has been measured.

## Requirements

- Go 1.26.6 or newer, as specified in `go.mod`.
- For PostgreSQL storage: a running PostgreSQL server, `psql`, and the `golang-migrate` CLI. The local setup was verified with PostgreSQL 18.6, pgx v5.11.0, and migrate v4.20.1.
- For caching: Redis, tested locally through the `redis:8` Docker image with go-redis v9.23.0. On Windows, Docker Desktop must be running its Linux-container engine.

## Run

From the project root, start with in-memory storage:

```powershell
$env:STORAGE_BACKEND = 'memory'
$env:REDIS_URL = ''
go run ./cmd/server
```

The server listens on `http://127.0.0.1:8080` and is accessible from the local computer. An unset or empty `STORAGE_BACKEND` also selects memory storage. For persistence, complete the PostgreSQL setup below before starting the server with that backend.

| Environment variable | Purpose |
| --- | --- |
| `STORAGE_BACKEND` | Empty or `memory` selects memory storage; `postgres` selects PostgreSQL. Other values fail startup. |
| `DATABASE_URL` | Required for PostgreSQL server mode. Ignored in memory mode. |
| `TEST_DATABASE_URL` | Enables real PostgreSQL integration tests. Separate from application configuration. |
| `REDIS_URL` | Nonempty enables Redis caching and requires `STORAGE_BACKEND=postgres`; unset or empty disables caching. |
| `TEST_REDIS_URL` | Enables Redis client and adapter integration tests independently of server configuration. |
| `MANAGEMENT_API_KEY` | Enables the protected DELETE route when nonempty. Unset or empty leaves deletion disabled; whitespace-only configuration fails startup. |

The application reads process environment variables. It does not load a `.env` file automatically.

## PostgreSQL setup on Windows

### Create a development database

Install PostgreSQL Server and Command Line Tools if needed. pgAdmin is optional; Stack Builder add-ons are not required. Use the [official Windows installer page](https://www.postgresql.org/download/windows/).

The examples below use the installed PostgreSQL 18 client and default port. Adjust the path and port for your installation:

```powershell
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -p 5432 -U postgres -d postgres -W
```

For a new setup, type these at the `psql` prompt, one at a time:

```sql
CREATE ROLE url_shortener_dev LOGIN;
```

Set its password interactively:

```text
\password url_shortener_dev
```

Then create the project database:

```sql
CREATE DATABASE url_shortener OWNER url_shortener_dev;
```

To exit psql, enter `\q`. If the role and database already exist, do not create them again.

The development role owns the database and runs migrations. It is not a PostgreSQL superuser. A deployed application can use a role with fewer permissions.

### Install the migration runner

From PowerShell:

```powershell
go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1

$migrationBin = go env GOBIN
if ([string]::IsNullOrWhiteSpace($migrationBin)) {
    $migrationBin = Join-Path (go env GOPATH) 'bin'
}

& (Join-Path $migrationBin 'migrate.exe') -help
```

This installs a separate tool without adding it to the application's `go.mod`. The project already records its pgx dependency in `go.mod` and `go.sum`.

### Configure the local connection

In the same PowerShell session, enter the password for `url_shortener_dev` without putting it in command history:

```powershell
$databasePassword = Read-Host 'Password for url_shortener_dev' -AsSecureString
$databaseCredential = [System.Net.NetworkCredential]::new('', $databasePassword)
$encodedDatabasePassword = [Uri]::EscapeDataString($databaseCredential.Password)

$localDatabaseURL = "postgres://url_shortener_dev:${encodedDatabasePassword}@localhost:5432/url_shortener?sslmode=disable"
```

URL encoding handles password characters such as `@`. The connection URL contains credentials. Do not print the connection URL. Do not commit it to Git. Do not share it.

`sslmode=disable` disables encryption. Use this setting only for the local development connection. Set the variables again in each new terminal.

### Apply and inspect migrations

From the project root:

```powershell
& (Join-Path $migrationBin 'migrate.exe') -path './migrations' -database $localDatabaseURL up
& (Join-Path $migrationBin 'migrate.exe') -path './migrations' -database $localDatabaseURL version
```

The migration runner should report version `1` without a dirty indication. If the database already has this migration, `up` reports `no change`. Run migrations separately. Server startup does not apply migrations.

Connect to inspect the schema:

```powershell
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -p 5432 -U url_shortener_dev -d url_shortener -W
```

At the `psql` prompt:

```text
\d urls
```

```sql
SELECT version, dirty FROM schema_migrations;
```

The `urls` table has an identity primary key and a unique `code`. Check constraints reject empty codes and destination URLs. The creation timestamp has a default value. The expiry timestamp can be null.

Both timestamps use `TIMESTAMPTZ` (`timestamp with time zone`). The session timezone controls their displayed values. The timestamps identify instants. A null expiry means that the link has no expiry. Lifecycle support uses the existing expiry column. No additional migration is necessary.

The down migration, `001_create_urls.down.sql`, removes the table and its data. The up migration restores the table structure. It does not restore deleted records.

Before you repair a dirty migration, inspect the actual schema. The `force` command changes version metadata without SQL execution. In this runner, `-1` means that no migration is applied. The negative argument requires `force -- -1`. Use this command only for recovery, not for normal setup.

### Start with PostgreSQL

After leaving `psql`, in the configured PowerShell session:

```powershell
$env:STORAGE_BACKEND = 'postgres'
$env:DATABASE_URL = $localDatabaseURL
go run ./cmd/server
```

Startup creates a shared pgx pool. It checks connectivity with a five-second context. If configuration or connectivity fails, startup returns an error. The server does not select memory storage instead. On normal shutdown, the pool closes after HTTP shutdown.

## Redis caching setup

### Start local Redis

Open Docker Desktop and wait for its engine to start. In PowerShell:

```powershell
docker info --format '{{.OSType}}'
```

Expect `linux`. For first-time setup, create the container:

```powershell
docker run --detach --name url-shortener-redis --publish 127.0.0.1:6379:6379 redis:8
```

For an already-created container, use this instead of repeating `docker run`:

```powershell
docker start url-shortener-redis
```

Check Redis with the client included inside the container:

```powershell
docker exec url-shortener-redis redis-cli ping
```

Expect `PONG`. The port mapping exposes Redis only on the Windows loopback address. You do not need a separate Windows `redis-cli` installation.

The container ping checks Redis inside the container. The Go integration test checks access through the host port.

### Enable caching in the Go server

In the same terminal containing the PostgreSQL connection setup:

```powershell
$env:STORAGE_BACKEND = 'postgres'
$env:DATABASE_URL = $localDatabaseURL
$env:REDIS_URL = 'redis://127.0.0.1:6379/0'
go run ./cmd/server
```

To enable deletion, configure `MANAGEMENT_API_KEY` in the server terminal before startup. Expect `Redis lookup caching configured`, then the server startup message. After configuration changes, restart the Go server.

The Redis client does not send a startup PING. Thus, the configuration message does not prove connectivity. Invalid Redis URL configuration stops startup. If Redis is unavailable during lookup, the server queries PostgreSQL.

To disable caching, set `REDIS_URL` to an empty string. Then restart the server. A Redis URL in memory mode stops startup.

### Cache policy

PostgreSQL is the primary storage. Creation writes directly to PostgreSQL. It does not read or write Redis. Metadata and redirect lookups use this sequence:

```text
Redis hit -> return cached record
Redis miss, read error, or invalid cached data -> query PostgreSQL
    database error -> preserve the error; do not cache it
    record found -> calculate TTL -> populate Redis when TTL is positive
```

The wrapper logs cache-write failures. It still returns a record retrieved from primary storage. Parent-request cancellation stops processing. A Redis timeout permits a PostgreSQL query if the parent context is active. The cache does not store missing-record results.

- The maximum cache time to live (TTL) is five minutes. Startup configuration sets this value.
- An expiring link uses the smaller of that maximum and its remaining lifetime when the TTL is calculated.
- TTLs are rounded down to milliseconds. Zero, negative, expired, and sub-millisecond results skip insertion rather than create a permanent key.
- Cache hits do not refresh TTLs; this is fixed TTL from insertion, not sliding expiry.
- Cached JSON includes the record's expiry. The HTTP handlers still check the actual deadline and return 410 when expired.
- An expired primary record can be returned for HTTP classification, but is not repopulated into Redis.

Each Redis operation in the wrapper has a 200 ms context limit. The client also has one-second connection, read, and write timeouts. It respects context deadlines and disables automatic command retries.

These separate limits do not guarantee a complete HTTP response within 200 ms. Pool waits, primary queries, and locks can add time.

### Namespaces and deletion consistency

Startup generates a fresh random namespace with `crypto/rand.Text`. Application keys have this form:

```text
url-shortener:v1:url:<run-namespace>:<code>
```

The namespace does not change short URLs. Each server run uses a new namespace and starts with an empty usable cache. It ignores entries from earlier runs. Old entries expire through their TTLs. Startup does not clear Redis.

Some adapter tests use an empty namespace. Application startup always supplies a namespace.

The wrapper holds a read lock during lookup and cache population. Deletion takes the write lock. It removes the Redis entry before primary deletion. This sequence prevents an active lookup from restoring the deleted mapping through the same wrapper.

If cache invalidation fails, the API returns 500. It does not attempt primary deletion. Reads can still query PostgreSQL during the Redis outage.

If primary deletion fails after invalidation, the cache remains empty. The next lookup checks the actual primary state. Removal of an absent Redis key succeeds.

This policy requires one Go server process and writes through its handlers. The lock does not coordinate separate processes. A run namespace does not invalidate entries in another active process.

Multiple servers and direct database writes require additional coordination. Concurrent cache misses can cause repeated primary queries. The server does not combine these requests to prevent a cache stampede.

## Endpoints

| Method | Path | Successful response |
| --- | --- | --- |
| POST | `/api/v1/urls` | 201 with `code`, `url`, `short_url`, and optional `expires_at` as JSON |
| GET | `/api/v1/urls/{code}` | 200 with `code`, `url`, and optional `expires_at` as JSON for an active link |
| GET | `/{code}` | 302 with the destination in the `Location` header |
| DELETE | `/api/v1/urls/{code}` | 204 with no body; requires configured management authentication |
| GET | `/healthz` | 200 with `OK` |
| GET | `/readyz` | 200 with `OK` when primary storage is ready; otherwise 503 with `not ready` |
| GET | `/` | 200 with `Hello, Go!` |

Unknown codes return 404. Expired metadata and redirect requests return 410 without a redirect destination. Successful create and metadata responses have `Content-Type: application/json`. Error responses contain plain text. Successful deletion has no body. Go GET route patterns also accept HEAD requests.

`ServeMux` selects the most specific matching pattern. The health route has priority over `/{code}`. The wildcard captures one path segment. The root pattern, `GET /{$}`, matches only `/`.

The health endpoint checks whether the HTTP server can respond. It does not check PostgreSQL. Readiness calls PostgreSQL `pool.Ping` with a one-second child context. In memory mode, readiness succeeds immediately.

Readiness checks connectivity, not the `urls` schema. It does not check Redis. Reads can use PostgreSQL during Redis outages, but deletion can fail. Readiness responses include `Cache-Control: no-store`.

## Create, inspect, and follow a short link

Run these commands in a second PowerShell terminal while the server is running:

```powershell
$response = Invoke-WebRequest -Uri 'http://127.0.0.1:8080/api/v1/urls' -Method Post -ContentType 'application/json' -Body '{"url":"https://example.com/articles/go"}'
$link = $response.Content | ConvertFrom-Json
$response.StatusCode
$link
```

Expect status 201 and a response resembling:

```json
{"code":"Ab12Cd34","url":"https://example.com/articles/go","short_url":"http://127.0.0.1:8080/Ab12Cd34"}
```

The code varies with each creation. Inspect the saved metadata:

```powershell
curl.exe -i "http://127.0.0.1:8080/api/v1/urls/$($link.code)"
```

Expect status 200 with the generated code and original URL. Inspect the redirect:

```powershell
curl.exe -i $link.short_url
```

Expect status 302 and `Location: https://example.com/articles/go`. Curl does not follow the redirect unless you add `-L`. The shortener itself does not fetch the destination.

Check a missing record and the basic endpoints:

```powershell
curl.exe -i http://127.0.0.1:8080/api/v1/urls/missing
curl.exe -i http://127.0.0.1:8080/missing
curl.exe -i http://127.0.0.1:8080/healthz
curl.exe -i http://127.0.0.1:8080/
```

Expect two 404 responses, followed by two 200 responses. The health body contains `OK`. The root body contains the greeting.

In memory mode, create a new link after each restart. PostgreSQL links remain available when the server connects to the same database.

## Custom aliases and expiry

Creation accepts optional `alias` and `expires_at` fields alongside the required destination URL:

| Field | Omitted or null | Supplied value |
| --- | --- | --- |
| `alias` | Generate a random code | Use the exact validated alias; an empty string is invalid |
| `expires_at` | No expiry | RFC 3339 timestamp with timezone, such as `2030-01-01T12:00:00Z` |

Aliases must contain 3 to 32 ASCII letters, digits, hyphens, or underscores. The stored alias keeps its capitalization. Aliases are case-sensitive in the tested storage setup. The reserved names are `healthz`, `readyz`, and `api`, regardless of capitalization.

The validator does not remove whitespace. Custom aliases and generated codes share one namespace. Before readiness registration, the developer checked for existing `readyz` aliases without a case distinction. No matching records existed. Validation does not change existing records.

An occupied alias returns 409. The server keeps the existing mapping and does not generate a random code instead. Alias creation makes one save attempt. Random allocation permits five attempts.

Expiry specifies an absolute deadline. It is not a TTL duration. Both creation paths copy the supplied timestamp. They convert the copy to UTC and truncate it to microsecond precision. The result must be later than the current time.

Malformed timestamps fail JSON decoding. Valid timestamps in the past fail service validation. Both failures return 400. Creation returns the normalized UTC deadline. Metadata returns the same instant, possibly with a different timezone offset after storage.

If a link has no expiry, the response omits `expires_at`. It does not return null for that field.

A link expires at its deadline. It remains expired after that instant. Metadata and redirects return 410 for expired links. Expiry does not remove the record or release its code. A new request for the same alias still conflicts.

### Try the lifecycle over HTTP

With the server running, create a uniquely named alias that expires in two minutes:

```powershell
$deadline = [DateTimeOffset]::UtcNow.AddMinutes(2)
$payload = @{
    url = 'https://example.com/expiry-demo'
    alias = "expiry-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
    expires_at = $deadline.ToString('o')
} | ConvertTo-Json -Compress

$response = Invoke-WebRequest -Uri 'http://127.0.0.1:8080/api/v1/urls' -Method Post -ContentType 'application/json' -Body $payload
$lifecycleLink = $response.Content | ConvertFrom-Json
$lifecycleURL = "http://127.0.0.1:8080/api/v1/urls/$($lifecycleLink.code)"
$response.StatusCode
$response.Content
```

Expect 201 with the alias and normalized expiry. Before the deadline:

```powershell
curl.exe -i $lifecycleURL
curl.exe -i $lifecycleLink.short_url
```

Expect 200 with metadata and 302 with the destination. After the returned expiry passes, repeat the same requests; both should return 410.

Attempt alias reuse without an expiry:

```powershell
$reusePayload = @{
    url = 'https://example.com/replacement'
    alias = $lifecycleLink.code
} | ConvertTo-Json -Compress

$reusePayload | curl.exe -i -H 'Content-Type: application/json' --data-binary '@-' http://127.0.0.1:8080/api/v1/urls
```

Expect 409. If you resend the original payload after its deadline, expect 400. Expiry validation occurs before the save attempt and alias-conflict check.

## Protected deletion

Deletion permanently removes an active or expired mapping. It releases the code for reuse. Success returns 204 with an empty body. A missing code returns 404 if the required storage and cache operations succeed. A repeated deletion also returns 404.

After deletion, metadata and redirect requests return 404. With caching enabled, Redis invalidation must succeed before primary deletion. If invalidation fails, the API returns 500. The primary record remains.

Startup registers the DELETE route only if `MANAGEMENT_API_KEY` is not empty. Without this configuration, DELETE on the metadata path returns 405. With this configuration, missing or invalid credentials return 401 with `WWW-Authenticate: Bearer`. The protected handler does not execute.

One shared management key protects deletion. Any holder can delete any link. The server does not assign links to individual users. Creation, metadata, redirects, health, and readiness remain public.

The middleware compares fixed-length SHA-256 hashes with `crypto/subtle.ConstantTimeCompare`. It does not log credentials.

### Enable deletion

Choose and privately retain a long random management key, separate from the database password. In the server terminal, enter it at a hidden prompt before starting or restarting Go:

```powershell
$managementKeyInput = Read-Host 'Management API key' -AsSecureString
$env:MANAGEMENT_API_KEY = [System.Net.NetworkCredential]::new('', $managementKeyInput).Password
go run ./cmd/server
```

Keep the storage configuration in the server terminal. The server reads the management key only at startup. Do not put the key in source code, command text, logs, or shared output.

The current listener uses local HTTP. Before network use, protect bearer credentials with HTTPS.

### Verify unauthorized and authorized deletion

In a second terminal, create a disposable link:

```powershell
$response = Invoke-WebRequest -Uri 'http://127.0.0.1:8080/api/v1/urls' -Method Post -ContentType 'application/json' -Body '{"url":"https://example.com/delete-check"}'
$deleteLink = $response.Content | ConvertFrom-Json
$deleteURL = "http://127.0.0.1:8080/api/v1/urls/$($deleteLink.code)"

curl.exe -i -X DELETE $deleteURL
curl.exe -i $deleteURL
```

Expect 401 followed by 200, proving the unauthorized request preserved the record. Enter the same key used by the server:

```powershell
$clientKeyInput = Read-Host 'Management API key' -AsSecureString
$clientKey = [System.Net.NetworkCredential]::new('', $clientKeyInput).Password
$managementHeaders = @{ Authorization = "Bearer $clientKey" }

$result = Invoke-WebRequest -Uri $deleteURL -Method Delete -Headers $managementHeaders
$result.StatusCode
$result.Content

curl.exe -i $deleteURL
curl.exe -i $deleteLink.short_url
```

Expect 204 with an empty body. Then expect 404 from both lookups. Client and server terminals have separate variables. A client key does not configure a server that is already running. During debugging, share status codes instead of credentials.

## Input rules and failure responses

Creation accepts one JSON object with a string field named `url` and optional `alias` and `expires_at` fields:

- The entire body is limited to 4 KiB (4096 bytes), including trailing whitespace.
- Malformed JSON, unknown fields, incorrect field types, and additional JSON values are rejected.
- The URL must parse successfully, use `http` or `https`, and contain a nonempty hostname.
- Empty or missing URLs, relative paths, and unsupported schemes are rejected.
- Validation checks URL structure without contacting the destination; it does not establish destination availability or safety.
- Supplied alias and expiry values must meet the lifecycle rules above; invalid input is rejected before saving.

| Status | Meaning |
| --- | --- |
| 400 | Invalid JSON body, destination URL, alias, or expiry |
| 401 | Missing or invalid credentials for configured management deletion |
| 404 | Unknown short code |
| 405 | Method not registered, including DELETE when management deletion is disabled |
| 409 | Requested custom alias is already occupied |
| 410 | Existing link has expired; applies to metadata and redirect access |
| 413 | Request body exceeds the size limit |
| 500 | Code generation, database access, required deletion invalidation, or another unexpected internal operation fails |
| 429 | Creation attempts exceed the shared per-process limit |
| 503 | All five code-allocation attempts collide, downstream work reports `context.DeadlineExceeded`, or the readiness check fails |

The decoder does not enforce the request Content-Type. Send `application/json` as shown in the examples.

## Security and operational safeguards

### Request IDs and structured logs

The server generates a new request ID with `crypto/rand.Text`. It replaces a client-supplied `X-Request-ID`. The response header and request context contain the generated ID. Use the ID to correlate requests and logs. It does not grant authorization.

Router-generated responses also receive IDs. Incomplete headers fail before handler execution. Those requests do not pass through the middleware.

The middleware order is:

```text
Request ID -> five-second request context -> request logging -> ServeMux
```

Logging runs inside the timeout wrapper. Thus, logging reads the request whose `Pattern` the mux sets. Completed requests produce JSON logs through `log/slog` on standard output. Each log contains `request_id`, `method`, `route`, and `duration_ms`.

Unmatched routes have an empty pattern. Very fast requests can have zero duration. The logger records route patterns, not raw URLs, query strings, bodies, or authorization headers. It does not record response status or byte counts. If downstream code panics, the completion log does not execute.

Startup, normal shutdown, Redis cleanup warnings, and top-level failures also use structured logging. Response-helper and cache-wrapper diagnostics still use the standard `log` package. Some diagnostic logs are not JSON and have no request ID.

### Creation rate limit

The creation limit permits ten attempts per one-minute window. All clients share this limit within one server process. The limiter runs before JSON decoding, validation, and storage. Invalid input also uses capacity.

Excess attempts return 429 with `creation rate limit exceeded`. These responses include `Cache-Control: no-store`. The create handler does not execute. The response has no `Retry-After` header.

The first window starts when the server constructs the limiter. After that window expires, the next attempt starts a new window. One mutex protects the limit check and counter increment. Other endpoints do not use this limit.

A server restart resets the limiter. Separate processes have separate limits. Requests near a window boundary can exceed ten within a rolling minute. The limiter does not distinguish individual clients.

To repeat the burst check with a fresh server and no earlier creation attempts:

```powershell
1..11 | ForEach-Object {
    curl.exe -s -o NUL -w "%{http_code}\n" `
        -H 'Content-Type: application/json' `
        --data-binary '{}' `
        http://127.0.0.1:8080/api/v1/urls
}
```

Expect ten 400 responses followed by 429 when all attempts occur in the same window. The empty payload reaches validation but creates no record. After the window expires, another identical attempt returns 400 again. Health and readiness remain available throughout.

### URL safety and secrets

The validator accepts absolute HTTP and HTTPS destinations with a hostname. It also accepts localhost and private-network addresses. This policy checks URL structure. It does not establish destination safety, ownership, or availability.

The server sends a redirect. It does not fetch the destination. URL previews and availability checks would introduce server-side request forgery (SSRF) risks. Such features require a separate design for DNS checks, redirects, and other SSRF defenses.

Configuration uses process environment variables. The examples use hidden prompts for secret input. Do not commit or share database URLs, Redis credentials, or management keys. Do not log credentials.

Hidden prompts keep typed secrets out of command history. The process environment still contains plaintext credentials. The application has no integrated secret manager. A shared management key protects deletion. The listener accepts only local connections.

Shared deployment and TLS configuration are future work.

### Readiness outage and recovery

With PostgreSQL selected and Go left running, compare:

```powershell
curl.exe --max-time 5 -i http://127.0.0.1:8080/readyz
curl.exe --max-time 5 -i http://127.0.0.1:8080/healthz
```

The developer stopped PostgreSQL while the Go server remained running. Readiness returned 503. Health continued to return 200. After PostgreSQL restarted, readiness returned 200 without a Go restart.

The readiness failure body contains `not ready`. It does not contain the internal database error. To repeat this check, use the service commands below.

## Storage and code generation

Startup selects one repository for all handlers. Memory storage uses a map with `sync.RWMutex` protection. Duplicate detection and insertion share one write lock. A competing creation request cannot overwrite an existing mapping.

PostgreSQL storage uses a shared `pgxpool.Pool` and parameterized SQL. The `urls_code_unique` constraint enforces code uniqueness across requests and processes. Only SQLSTATE `23505` for this constraint becomes `errCodeExists`.

The repository translates `pgx.ErrNoRows` into `errURLNotFound`. Deletion checks `RowsAffected()` to detect an absent record. A DELETE with no matching rows is valid SQL, but the API reports a missing record. Other database errors retain their identity through wrapping.

The nullable `expires_at` column stores optional expiry. Repository lookup returns expired records. Metadata and redirect handlers apply the 410 policy. Deletion can remove expired records.

Codes use six random bytes from `crypto/rand`, encoded as eight URL-safe Base64 characters without padding. Randomness does not guarantee uniqueness. Creation retries with a new code after a collision, up to five attempts total.

The create-handler factory receives a repository and a generator function. It constructs one service for subsequent requests. The service allocates codes and saves records. Tests use controlled generators to produce collisions and allocation failure. Production uses the random generator.

A record is saved before the create response is written. If response writing fails, the record can exist even though the client did not receive confirmation.

## Application structure

The code remains in the `cmd/server` package. Each file has a specific responsibility. Add packages only when necessary.

| File | Responsibility |
| --- | --- |
| `main.go` | Select storage from environment configuration, register routes, and manage server startup and shutdown |
| `repository.go` | Define `urlRepository`, narrow `urlDeleter`, and composed `urlStore`; check both implementations |
| `store.go` | Define URL records and implement concurrency-safe in-memory storage |
| `database.go` | Create a pgx pool and verify connectivity with a bounded ping |
| `postgres_store.go` | Implement PostgreSQL saves, lookups, expiry persistence, and deletion with error translation |
| `cache.go` | Define the `urlCache` interface and cache-miss error |
| `cache_ttl.go` | Calculate finite, expiry-aware TTLs at millisecond precision |
| `redis_client.go` | Parse Redis connection configuration and configure network timeouts and command retries |
| `redis_cache.go` | Encode/decode records, construct namespaced keys, and implement Redis get/set/delete |
| `cached_store.go` | Add cache-aside lookup, primary-only saves, and coordinated invalidation/deletion |
| `service.go` | Validate destinations, aliases, and expiry; allocate random or custom codes and save records |
| `create_handler.go` | Decode create requests, call the service, and build HTTP responses |
| `metadata_handler.go` | Look up records and return metadata as JSON |
| `redirect_handler.go` | Look up records and return redirects |
| `delete_handler.go` | Delete a record through `urlDeleter` and return 204 or a mapped error |
| `auth.go` | Validate management credentials before calling the protected handler |
| `request_id.go` | Generate response request IDs and propagate them through context |
| `request_logging.go` | Log completed requests with IDs, methods, route patterns, and durations |
| `request_timeout.go` | Derive and clean up a request-scoped timeout context |
| `readiness_handler.go` | Run an injected primary-storage check with a one-second timeout |
| `create_limiter.go` | Enforce a mutex-protected fixed-window creation limit |
| `create_rate_limit.go` | Reject excess creation attempts before the protected handler executes |
| `alias.go` | Enforce custom-alias characters, length, and reserved names |
| `expiry.go` | Normalize and validate deadlines and check exact expiry boundaries |
| `request.go` | Decode one JSON request with a body-size limit and unknown-field rejection |
| `response.go` | Write JSON responses and map service errors to HTTP errors |
| `validation.go` | Enforce the destination URL policy |
| `code.go` | Generate random, URL-safe short codes |
| `migrations/001_create_urls.*.sql` (project root) | Define schema creation and its rollback |

Creation follows this flow:

```text
HTTP request -> decoder -> service -> repository
                             |
                       code generator

Service result or error -> HTTP handler -> response helper -> HTTP response
```

Handlers depend on `urlRepository`, rather than the map implementation. Go checks interface satisfaction implicitly from matching method signatures. Tests can therefore substitute a fake repository to exercise storage failures.

All storage operations accept `context.Context`. Creation passes `r.Context()` through the service to storage. Metadata, redirects, and deletion also pass the request context to storage.

Lookup returns `(urlRecord, error)`. The error distinguishes absent records, cancellation, and storage failures. The delete handler needs only `urlDeleter`. Startup selects the composed `urlStore` interface.

The connection helper cancels its setup context after connection setup. Later queries use the context supplied by their caller.

The service has no dependency on HTTP. It returns records or errors, preserving unexpected generator and repository errors with `%w`. The response helper recognizes wrapped application errors with `errors.Is`:

- `errInvalidURL` maps to 400.
- `errInvalidAlias` and `errInvalidExpiry` map to 400.
- `errAliasTaken` maps to 409.
- `errCodeAllocationExhausted` maps to 503.
- `errURLNotFound` maps to 404.
- `errURLExpired` maps to 410.
- `context.DeadlineExceeded`, including wrapped errors, maps to 503 with `request deadline exceeded` when reported through the response helper.
- Unexpected service failures are logged internally and map to a generic 500 response.

Request-decoding errors remain in the create handler: oversized bodies map to 413 and other decoding failures to 400. Redirect and metadata handlers currently read from the repository directly.

If caching is enabled, startup wraps PostgreSQL storage before handler construction. The wrapper implements `urlStore`. Handlers do not depend on Redis APIs.

The wrapper logs cache-read and cache-write failures. A successful primary lookup can still return successfully. A deletion invalidation failure returns an error. The generic HTTP mapping returns 500 for that error.

`writeJSON` sets Content-Type and commits the selected status before encoding. It logs encoding and write failures. After commitment, the helper cannot replace the response status.

## Development checks

Run these commands yourself from the project root:

```powershell
go fmt ./cmd/server
go vet ./...
go test -race -v ./...
```

### PostgreSQL integration tests

Use a development or dedicated test database with migration 1 applied. In the session containing `$localDatabaseURL`:

```powershell
$env:TEST_DATABASE_URL = $localDatabaseURL
go test -race -count=1 -v ./...
```

If that connection variable is not populated in this terminal, repeat the password-prompt setup above first. Integration tests skip when `TEST_DATABASE_URL` is absent. Look for PASS rather than SKIP on these tests:

- `TestOpenPostgresIntegration`
- `TestPostgresStoreSaveLookupAndCollisionIntegration`
- `TestPostgresStoreConcurrentInsertIntegration`
- `TestPostgresStoreExpiryRoundTripIntegration`
- `TestPostgresStoreDeleteIntegration`

The connection test runs `SELECT 1`. Repository tests create records with unique names. Cleanup removes those records, not the entire table. The concurrent test expects one successful insert and fifteen collisions for one code.

Cleanup uses a new timeout context and completes before pool closure. The `-count=1` flag prevents reuse of cached test results.

### Redis integration tests

Keep Docker Desktop and the Redis container running, then configure the test terminal:

```powershell
$env:TEST_REDIS_URL = 'redis://127.0.0.1:6379/0'
go test -race -count=1 -v ./...
```

Keep `TEST_DATABASE_URL` populated too to exercise both external services in one run. Redis integration tests skip when `TEST_REDIS_URL` is absent. Confirm PASS rather than SKIP for:

- `TestRedisClientIntegration`: host-to-Redis PING.
- `TestRedisCacheRoundTripIntegration`: misses, JSON/expiry round trip, positive bounded TTL, idempotent deletion, and skipped zero/negative/sub-millisecond TTLs.
- `TestRedisCacheNamespaceIntegration`: the same code can hold different values in separate namespaces, and deleting one does not affect the other.

The tests use generated keys and namespaces. They remove their own entries during cleanup. They do not clear Redis. Cache-wrapper unit tests use test doubles and need no external services.

Automated tests cover:

- Store save/lookup, missing records, collision preservation, and concurrent access.
- Valid and invalid destination URLs.
- JSON decoding, unknown fields, trailing values, malformed input, and oversized bodies, including trailing whitespace.
- Successful creation, rejected requests without storage changes, forced collision retries, and retry exhaustion.
- Service behavior independent of HTTP: success, validation before generation, generator failure, repository failure, collision retries, and allocation exhaustion.
- Preservation of wrapped errors with `errors.Is`, plus HTTP error mapping through a fake failing repository.
- Exactly one JSON value in the successful create response, guarding against duplicate response writes.
- Metadata responses and redirects for known and missing codes.
- PostgreSQL connectivity, save/lookup, missing-record translation, duplicate-code preservation, and concurrent uniqueness enforcement.
- Metadata and redirect handlers returning generic 500 responses without a redirect destination when repository lookup fails.
- Alias length boundaries, characters, reserved names, capitalization preservation, conflicts, and no random fallback.
- Optional alias/expiry decoding, including omitted versus explicit empty values, timestamp offsets, and invalid types.
- Expiry normalization, caller-input preservation, past-deadline rejection, and before/at/after deadline boundaries.
- Expiry returned and stored through both creation paths, PostgreSQL timestamp round trips, active metadata fields, and omission for permanent links.
- Metadata and redirects returning 410 for expired links while retaining records.
- Memory and PostgreSQL deletion, missing records, preservation of unrelated records, and reuse of deleted codes.
- Delete-handler success, expiry-independent deletion, empty 204 bodies, and storage failures.
- Authentication rejection, empty-key configuration, handler execution control, and actual record preservation during unauthorized deletion.
- Cache TTL maximums, expiry boundaries, millisecond truncation, and skip-insertion cases.
- Redis adapter operations, expiry serialization, finite TTLs, and namespace isolation.
- Cache hits avoiding primary lookup and TTL refresh, fallback on misses/read errors, and successful reads despite cache-population errors.
- Primary-error identity preservation, no cache population for primary failures, canceled requests, and primary-only saves.
- Deletion operation order, invalidation failure preventing primary deletion, primary failure, and missing-record results through the wrapper.
- Request-ID propagation, replacement of client-supplied IDs, context-value preservation, and parent cancellation.
- Structured request logs for matched and unmatched routes through the full middleware chain, including ID correlation and duration-field presence.
- Startup configuration errors returning from `run` for unknown backends and Redis configured with memory storage.
- Readiness success, storage failure, request cancellation, deadline presence, safe response bodies, and non-cacheable responses.
- Creation-limit configuration, sequential window boundaries, exactly ten allowed attempts under concurrent load, middleware rejection without downstream execution, and reset behavior.
- Request-timeout inheritance, preservation of context values, child cleanup without canceling the parent, immediate expiry, and parent cancellation.
- Direct and wrapped deadline errors mapping to a safe 503 response.

The developer reports passing tests with the race detector and no vet findings. The reported runs included real PostgreSQL and Redis integration tests.

Manual HTTP checks covered creation, aliases, metadata, redirects, expiry, and protected deletion. They also covered cache failure, recovery, invalidation, and code reuse. Module 6 checks covered readiness recovery, rate-limit reset, request logging, and header timeout.

Direct handler tests do not automatically check route registration. Race checks apply to the executions tested. They do not prove correctness for all concurrent executions.

### Coverage

Generate and inspect statement coverage in PowerShell:

```powershell
go test -race "-coverprofile=coverage.out" ./...
go tool cover "-func=coverage.out"
```

Quote the complete flags as shown to avoid argument-parsing problems in PowerShell. The generated `coverage.out` file is ignored by Git.

The Module 2 report dated October 21, 2026 showed **73.6% total statement coverage**. All tests passed. The service, repository, handlers, decoder, validator, and error mapping each showed 100%. `generateCode` and `writeJSON` each showed 75%. `main` showed 0%.

This report predates PostgreSQL, lifecycle, Redis, and Module 6 changes. It does not measure the current code. For a new report, set both `TEST_DATABASE_URL` and `TEST_REDIS_URL` before the coverage command.

No current coverage figure has been reported. The historical report did not cover startup, shutdown, and some error branches. The developer checked startup and basic shutdown manually.

Coverage identifies executed statements. Assertions check results. Full statement coverage does not prove that all edge cases work.

## Current limitations

- Memory storage has no capacity limit and is cleared on process exit. Persistence requires selecting the PostgreSQL backend.
- The listen address and public short-link base URL are fixed in application setup.
- Analytics and the optional update endpoint are not implemented yet.
- All clients share one creation limit within a process. Restart resets the limit. Window boundaries permit bursts. Separate processes have separate limits.
- Cache coordination supports the current single-process setup, not distributed invalidation across multiple active servers or external database writers.
- Redis outages allow read fallback but prevent deletion while invalidation is unavailable. Cache stampede prevention and performance benchmarks are not implemented.
- Authentication is one shared management key protecting deletion; there is no per-user ownership or credential rotation mechanism.
- The database stores an ID and creation timestamp, but metadata currently exposes only the code, original URL, and optional expiry.
- Expired records are retained without automatic cleanup. Their codes remain reserved until explicit deletion, which releases the code for reuse.
- `/healthz` does not test dependencies; `/readyz` checks primary connectivity but not schema or Redis. Unexpected database failures still map to 500; reported request deadlines map to 503.
- Request deadlines are cooperative. Blocking body reads and code that ignores context are not forcibly stopped by the five-second middleware deadline.
- Request access logs do not include response status or byte count. Some internal diagnostics still use unstructured logging.

## Manual persistence and outage checks

The developer reports verifying the following against local PostgreSQL, with Redis caching disabled for the original database-outage check:

1. The developer created a PostgreSQL link and restarted only the Go server. The same code returned metadata status 200 and redirect status 302.
2. The developer stopped PostgreSQL while Go remained running. Metadata and redirects returned 500. The health endpoint continued to return 200.
3. The developer restarted PostgreSQL without a Go restart. The original link became available again. This check verified connection-pool recovery and stored-record persistence.

For this Windows installation, the service is `postgresql-x64-18`. Confirm the service name before testing on another installation:

```powershell
Get-Service -Name '*postgres*' | Select-Object Name, Status, DisplayName
```

Use a separate administrator PowerShell terminal for these commands. Stopping PostgreSQL affects all applications that use this instance. After the outage check, restore the service:

```powershell
Stop-Service -Name 'postgresql-x64-18'
# Inspect lookup failures in the request terminal, then restore PostgreSQL.
Start-Service -Name 'postgresql-x64-18'
```

Keep Go running in PostgreSQL mode during the check. Use `curl.exe --max-time 10 -i` to limit client waiting. The curl timeout limits the client. It does not set a database deadline. These service restart checks are manual.

### Redis behavior verified manually

With PostgreSQL and Redis caching enabled, the developer verified:

1. Creation writes only to primary storage. The first lookup creates a cache entry in the run namespace. Repeated lookup returns the correct destination while PTTL decreases.
2. During a Redis outage, metadata and redirects still use PostgreSQL. Authorized deletion returns 500 and preserves the primary record. Redis recovery does not require a Go restart.
3. Successful deletion returns 204 and removes the cache entry. Metadata and redirects then return 404. Code reuse returns the new destination instead of old cache data.
4. A two-minute link receives a cache TTL below 120000 ms. After expiry, Redis has no key. Metadata and redirects return 410. Further expired lookups do not restore the cache entry.

For a fresh link created using the earlier request example, inspect the cache from the request terminal:

```powershell
$metadataURL = "http://127.0.0.1:8080/api/v1/urls/$($link.code)"
$pattern = "url-shortener:v1:url:*:$($link.code)"
curl.exe -i $metadataURL

$redisKey = docker exec url-shortener-redis redis-cli --scan --pattern $pattern
$redisKey
docker exec url-shortener-redis redis-cli PTTL $redisKey
```

Use a code created during the current server run. Its pattern should identify the active key. PTTL uses milliseconds. A permanent link should have a positive PTTL no greater than 300000. An expiring link uses its shorter remaining lifetime. Cache hits do not extend the TTL.

To repeat the Redis outage check, keep Go and PostgreSQL running:

```powershell
docker stop url-shortener-redis
curl.exe --max-time 10 -i $metadataURL
curl.exe --max-time 10 -i $link.short_url
```

Expect 200 and 302 for an active link. Restore Redis even if a request differs from expectations:

```powershell
docker start url-shortener-redis
docker exec url-shortener-redis redis-cli ping
```

Use the deletion procedure to check failure and recovery. Expect 500 if invalidation fails. Expect 204 after recovery. These functional checks do not measure latency or throughput. Performance measurement is future work.

## Shutdown and timeouts

Press **Ctrl+C** in the server terminal. Expected log messages include:

```text
Shutdown requested
Server stopped cleanly
```

Shutdown stops new connections. Active requests have up to five seconds to finish. If shutdown exceeds this deadline, the process logs an error and exits. Unfinished requests can be interrupted. A second Ctrl+C can stop the process during shutdown.

After shutdown, this command should fail to connect:

```powershell
curl.exe -i http://127.0.0.1:8080/healthz
```

The configured limits are:

| Limit | Duration | Scope |
| --- | --- | --- |
| `ReadHeaderTimeout` | 5 seconds | Reading request headers |
| `ReadTimeout` | 10 seconds | Reading the complete request, including its body |
| `WriteTimeout` | 15 seconds | Response-write deadline, reset when request headers are read |
| `IdleTimeout` | 60 seconds | Waiting for the next request on a keep-alive connection |
| Request context | 5 seconds | Cooperative downstream work after middleware begins |
| Readiness context | 1 second | Primary-storage connectivity check |
| Shutdown context | 5 seconds | Waiting for active HTTP requests to finish |

Connection timeouts and context deadlines have different purposes. A write deadline does not automatically cancel a PostgreSQL query. Handlers pass the request context to storage. Context-aware operations can stop at its deadline. Readiness uses an earlier one-second deadline.

The timeout middleware does not write a response automatically. It does not forcibly stop code. The response helper maps reported `context.DeadlineExceeded` errors to 503 before response commitment. A timed-out write can still have committed in primary storage.

Redis child contexts derive from the request context. Primary queries after a cache failure retain the outer request deadline. Locks do not observe context cancellation. Thus, the deadline does not guarantee a maximum total processing time.

The `run(logger)` function owns resources. It returns startup, serving, and shutdown errors. Its deferred cleanup executes before `main` logs the error and exits with code 1.

On normal shutdown, the Redis client and PostgreSQL pool close after HTTP shutdown. If graceful shutdown times out, cleanup still executes as `run` returns. This cleanup can interrupt active work. Only `main` exits the process.

The developer sent incomplete headers through a real TCP connection. The server closed the connection after approximately five seconds. `ReadByte` returned `-1` after 4.9995 seconds. A subsequent health request returned 200.

Direct handler tests do not check connection timeouts. Slow-body, slow-response, and idle-connection behavior have no separate verification.

The developer checked basic shutdown and connection refusal manually. Completion of an active request during shutdown has not been verified.

## Learning workflow

The developer types project code, configuration, tests, and migrations. The developer runs commands, tests, and Git operations. The assistant explains concepts and reviews code.

Use [ASD-STE100](https://www.asd-ste100.org/assets/files/ASD-STE100_ISSUE9.pdf) writing principles for documentation. Write short sentences. Give one instruction in each sentence. Use consistent technical terms. Preserve exact code identifiers, commands, and protocol values. Full compliance requires a separate review against the approved dictionary.

For the Module 6 update, the developer authorized direct edits to `README.md` only. Other files remain under the manual learning rules. The developer reviews the documentation before the milestone commit.

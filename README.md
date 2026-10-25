# Go URL Shortener

A learning project built by hand with Go and the standard-library `net/http` package. Follow [roadmap.md](roadmap.md) for the planned modules and [AGENTS.md](AGENTS.md) for the mentoring workflow.

The current implementation creates short links, redirects them, and returns their metadata using either concurrency-safe in-memory storage or PostgreSQL. It also provides a health endpoint and graceful shutdown. PostgreSQL records survive Go server restarts; memory records do not.

Implementation and verification cover Modules 0 through 3: HTTP foundations, the in-memory MVP, service and repository boundaries, and PostgreSQL persistence. URL lifecycle behavior is the next roadmap module.

## Requirements

- Go 1.26.6 or newer, as specified in `go.mod`.
- For PostgreSQL storage: a running PostgreSQL server, `psql`, and the `golang-migrate` CLI. The local setup was verified with PostgreSQL 18.6, pgx v5.11.0, and migrate v4.20.1.

## Run

From the project root, start with in-memory storage:

```powershell
$env:STORAGE_BACKEND = 'memory'
go run ./cmd/server
```

The server listens on `http://127.0.0.1:8080` and is accessible from the local computer. An unset or empty `STORAGE_BACKEND` also selects memory storage. For persistence, complete the PostgreSQL setup below before starting the server with that backend.

| Environment variable | Purpose |
| --- | --- |
| `STORAGE_BACKEND` | Empty or `memory` selects memory storage; `postgres` selects PostgreSQL. Other values fail startup. |
| `DATABASE_URL` | Required for PostgreSQL server mode. Ignored in memory mode. |
| `TEST_DATABASE_URL` | Enables real PostgreSQL integration tests. Separate from application configuration. |

These are process environment variables; the application does not automatically load a `.env` file.

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

Exit with `\q`. Skip role and database creation when they already exist. The development role owns this database and runs migrations; it is not a PostgreSQL superuser. A deployed application can use a more restricted runtime role.

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

URL encoding handles password characters such as `@`. The resulting connection URL contains credentials: do not print, commit, or share it. `sslmode=disable` disables encryption and is used here only for the local development connection. Variables must be set again in a new terminal session.

### Apply and inspect migrations

From the project root:

```powershell
& (Join-Path $migrationBin 'migrate.exe') -path './migrations' -database $localDatabaseURL up
& (Join-Path $migrationBin 'migrate.exe') -path './migrations' -database $localDatabaseURL version
```

The current migration set should report version `1` without a dirty indication. On an already migrated database, `up` reports `no change`. Migrations are run separately; starting the Go server does not apply them automatically.

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

The `urls` table has an identity primary key, a unique `code`, nonempty checks for the code and original URL, a default creation timestamp, and a nullable expiry timestamp. Both timestamps use `TIMESTAMPTZ` (`timestamp with time zone`). Session timezone controls display; the values represent instants. A null expiry means no expiration. Expiry behavior is not implemented yet.

`001_create_urls.down.sql` drops the table and its data. Reapplying the up migration recreates the structure, not the deleted records. A dirty migration requires inspecting the actual schema before repairing tracking: `force` changes version metadata without running SQL. In this runner, no applied migration is represented by `-1`, not `0`; passing a negative force argument requires `force -- -1`. That is a recovery mechanism, not part of normal setup.

### Start with PostgreSQL

After leaving `psql`, in the configured PowerShell session:

```powershell
$env:STORAGE_BACKEND = 'postgres'
$env:DATABASE_URL = $localDatabaseURL
go run ./cmd/server
```

Startup creates a shared pgx pool and checks connectivity with a five-second context. Startup fails if configuration or connectivity fails; it does not silently fall back to memory storage. On normal shutdown, the pool closes after the HTTP server finishes shutting down.

## Endpoints

| Method | Path | Successful response |
| --- | --- | --- |
| POST | `/api/v1/urls` | 201 with `code`, `url`, and `short_url` as JSON |
| GET | `/api/v1/urls/{code}` | 200 with `code` and `url` as JSON |
| GET | `/{code}` | 302 with the destination in the `Location` header |
| GET | `/healthz` | 200 with `OK` |
| GET | `/` | 200 with `Hello, Go!` |

Unknown codes return 404. Successful API responses use `Content-Type: application/json`; error responses are plain text. Go's GET route patterns also accept HEAD requests.

`ServeMux` selects the most specific matching pattern. The literal health route takes precedence over `/{code}`, which captures one path segment. The root route uses `GET /{$}` to match only `/`.

The health endpoint confirms that the HTTP server can respond; it does not check database availability.

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

Expect 404, 404, 200 with `OK`, and 200 with the greeting, respectively. In memory mode, create a fresh link after restarting. In PostgreSQL mode, existing links remain available when the server reconnects to the same database.

## Input rules and failure responses

Creation accepts one JSON object with a string field named `url`:

- The entire body is limited to 4 KiB (4096 bytes), including trailing whitespace.
- Malformed JSON, unknown fields, incorrect field types, and additional JSON values are rejected.
- The URL must parse successfully, use `http` or `https`, and contain a nonempty hostname.
- Empty or missing URLs, relative paths, and unsupported schemes are rejected.
- Validation checks URL structure without contacting the destination; it does not establish destination availability or safety.

| Status | Meaning |
| --- | --- |
| 400 | Invalid JSON body or rejected URL |
| 404 | Unknown short code |
| 413 | Request body exceeds the size limit |
| 500 | Code generation, database access, or another unexpected internal operation fails |
| 503 | All five code-allocation attempts encounter collisions |

The decoder does not currently enforce the request's `Content-Type`; clients should send `application/json` as shown above.

## Storage and code generation

One repository is selected at startup and shared by all handlers. Memory mode uses a map protected by `sync.RWMutex`. Duplicate-code detection and insertion share a write lock, so an existing mapping cannot be overwritten by a competing create operation.

PostgreSQL mode uses a shared `pgxpool.Pool`. Inserts and lookups use parameterized SQL. The database's `urls_code_unique` constraint enforces code uniqueness across requests and server processes. The repository translates SQLSTATE `23505` for that specific constraint into `errCodeExists`; it translates `pgx.ErrNoRows` into `errURLNotFound`. Other database errors are wrapped and returned rather than misreported as missing records.

Codes use six random bytes from `crypto/rand`, encoded as eight URL-safe Base64 characters without padding. Randomness does not guarantee uniqueness. Creation retries with a new code after a collision, up to five attempts total.

The create-handler factory accepts a repository and generator function, then constructs one service reused by its returned handler. The service performs allocation and storage. Tests supply controlled generators to force collisions and verify retry exhaustion reliably. Production uses the random generator.

A record is saved before the create response is written. If response writing fails, the record can exist even though the client did not receive confirmation.

## Application structure

The code remains in the `cmd/server` package. Responsibilities are separated into focused files; additional packages will be introduced when useful.

| File | Responsibility |
| --- | --- |
| `main.go` | Select storage from environment configuration, register routes, and manage server startup and shutdown |
| `repository.go` | Define the context-aware `urlRepository` interface and check that both stores satisfy it |
| `store.go` | Define URL records and implement concurrency-safe in-memory storage |
| `database.go` | Create a pgx pool and verify connectivity with a bounded ping |
| `postgres_store.go` | Implement PostgreSQL inserts and lookups with parameterized SQL and error translation |
| `service.go` | Validate destinations, generate codes, retry collisions, and save records |
| `create_handler.go` | Decode create requests, call the service, and build HTTP responses |
| `metadata_handler.go` | Look up records and return metadata as JSON |
| `redirect_handler.go` | Look up records and return redirects |
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

Both repository methods accept `context.Context`. The create handler passes `r.Context()` through the service to saving; metadata and redirect handlers pass it to lookup. Lookup returns `(urlRecord, error)`, distinguishing missing records from cancellation and storage failures. The connection helper's temporary context is canceled after setup; later queries use their caller's context.

The service has no dependency on HTTP. It returns records or errors, preserving unexpected generator and repository errors with `%w`. The response helper recognizes wrapped application errors with `errors.Is`:

- `errInvalidURL` maps to 400.
- `errCodeAllocationExhausted` maps to 503.
- `errURLNotFound` maps to 404.
- Unexpected service failures are logged internally and map to a generic 500 response.

Request-decoding errors remain in the create handler: oversized bodies map to 413 and other decoding failures to 400. Redirect and metadata handlers currently read from the repository directly.

`writeJSON` sets the content type and commits the caller-selected status before encoding. Encoding or write failures are logged; the helper cannot replace an already committed response with a new error status.

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

The connection test runs `SELECT 1`. Repository tests insert uniquely named test records and delete them during cleanup; they do not clear the table. The concurrent test expects one successful insert and fifteen collisions for a shared code. Cleanup uses a fresh timeout context and runs before pool closure. `-count=1` requests a fresh run rather than a cached result.

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

The developer reports that these tests pass with the race detector enabled and that vet reports no issues. Manual HTTP checks have verified creation, metadata lookup, redirects, missing records, and the basic endpoints. Handler tests call handlers directly; they do not automatically verify route registration. Passing race checks cover the executions tested, rather than proving every possible concurrent execution correct.

### Coverage

Generate and inspect statement coverage in PowerShell:

```powershell
go test -race "-coverprofile=coverage.out" ./...
go tool cover "-func=coverage.out"
```

Quote the complete flags as shown to avoid argument-parsing problems in PowerShell. The generated `coverage.out` file is ignored by Git.

The Module 2 run shared on October 21, 2026 reported **73.6% total statement coverage**, with all tests passing. At that point, the service, repository implementation, handlers, request decoder, validator, and service-error mapping each reported 100%. `generateCode` and `writeJSON` each reported 75%; `main` reported 0%. This is historical coverage from before PostgreSQL support, not a measurement of the current code. Run the commands above with `TEST_DATABASE_URL` set for an updated report that includes integration tests.

Startup and shutdown remain outside the automated suite, and error branches in the random generator and JSON writer remain uncovered. Basic shutdown has been checked manually. Coverage records executed statements; assertions are still needed to establish correct results, and 100% coverage does not prove every edge case is handled.

## Current limitations

- Memory storage has no capacity limit and is cleared on process exit. Persistence requires selecting the PostgreSQL backend.
- The listen address and public short-link base URL are fixed in application setup.
- Authentication, rate limiting, expiry, custom aliases, Redis caching, and analytics are not implemented yet.
- Metadata currently contains only the short code and original URL.
- The database stores an ID and timestamps, but the current Go record and API expose only the code and original URL. Application-created rows leave expiry null.
- `/healthz` does not test database readiness. Unexpected database failures currently map to 500 rather than a separately classified availability response.
- Request contexts propagate cancellation, but database operations do not yet receive an additional application-defined query deadline.

## Manual persistence and outage checks

The developer reports verifying the following against local PostgreSQL:

1. Create a link in PostgreSQL mode, stop and restart only the Go server, and query the same code. Metadata still returns 200 and the short URL returns 302 with the original destination.
2. Leave Go running and temporarily stop the local PostgreSQL service. Metadata and redirect requests return 500, while `/healthz` continues returning 200.
3. Restart PostgreSQL without restarting Go. The existing link becomes readable and redirects again, verifying pool recovery and persistence across the database service restart.

For this Windows installation, the service is `postgresql-x64-18`. Confirm the service name before testing on another installation:

```powershell
Get-Service -Name '*postgres*' | Select-Object Name, Status, DisplayName
```

In a separate administrator PowerShell, these commands stop and restore that local service. Stopping it affects all applications using that instance; restore it after checking the outage:

```powershell
Stop-Service -Name 'postgresql-x64-18'
# Inspect lookup failures in the request terminal, then restore PostgreSQL.
Start-Service -Name 'postgresql-x64-18'
```

The Go server must remain running in PostgreSQL mode throughout the outage/recovery check. The request examples above can be reused with `curl.exe --max-time 10 -i` to bound client waiting. Curl's timeout is a client limit, not a server query deadline. These restart and outage checks are manual, not automated service-control tests.

## Shutdown and timeouts

Press **Ctrl+C** in the server terminal. Expected log messages include:

```text
Shutdown requested
Server stopped cleanly
```

The server stops accepting new connections and gives active requests up to five seconds to finish. If shutdown exceeds that deadline, the process logs an error and exits, potentially interrupting unfinished requests. A second Ctrl+C can terminate the process during shutdown.

After shutdown, this command should fail to connect:

```powershell
curl.exe -i http://127.0.0.1:8080/healthz
```

Request headers have a separate five-second read timeout. This does not limit request-body reading or handler execution.

Basic shutdown and connection refusal afterward have been checked manually. Allowing an active request to finish during shutdown has not yet been verified.

## Learning workflow

The developer types all project code and runs commands, tests, and Git operations. The assistant explains concepts, reviews code, and helps debug. The developer has authorized the assistant to write `README.md` and `.gitignore`.

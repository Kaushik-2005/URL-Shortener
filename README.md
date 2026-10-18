# Go URL Shortener

A learning project built by hand with Go and the standard-library `net/http` package. Follow [roadmap.md](roadmap.md) for the planned modules and [AGENTS.md](AGENTS.md) for the mentoring workflow.

The current implementation creates short links, redirects them, and returns their metadata using concurrency-safe in-memory storage. It also provides a health endpoint and graceful shutdown. Records are lost whenever the process stops.

## Requirements

- Go 1.26.6 or newer, as specified in `go.mod`.

## Run

From the project root:

```powershell
go run ./cmd/server
```

The server listens on `http://127.0.0.1:8080` and is accessible from the local computer.

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

Expect 404, 404, 200 with `OK`, and 200 with the greeting, respectively. Create a fresh link after restarting the server because previous records no longer exist.

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
| 500 | Code generation or another unexpected internal operation fails |
| 503 | All five code-allocation attempts encounter collisions |

The decoder does not currently enforce the request's `Content-Type`; clients should send `application/json` as shown above.

## Storage and code generation

One store is created at startup and shared by all handlers. It holds a map protected by `sync.RWMutex`. Duplicate-code detection and insertion share a write lock, so an existing mapping cannot be overwritten by a competing create operation.

Codes use six random bytes from `crypto/rand`, encoded as eight URL-safe Base64 characters without padding. Randomness does not guarantee uniqueness. Creation retries with a new code after a collision, up to five attempts total.

The create handler accepts a generator function so tests can force collisions and verify retry exhaustion reliably. Production uses the random generator.

A record is saved before the create response is written. If response writing fails, the record can exist even though the client did not receive confirmation.

## Development checks

Run these commands yourself from the project root:

```powershell
go fmt ./cmd/server
go vet ./...
go test -race -v ./...
```

Automated tests cover:

- Store save/lookup, missing records, collision preservation, and concurrent access.
- Valid and invalid destination URLs.
- JSON decoding, unknown fields, trailing values, malformed input, and oversized bodies, including trailing whitespace.
- Successful creation, rejected requests without storage changes, forced collision retries, and retry exhaustion.
- Metadata responses and redirects for known and missing codes.

The developer reports that these tests pass with the race detector enabled and that vet reports no issues. Manual HTTP checks have verified creation, metadata lookup, redirects, missing records, and the basic endpoints. Handler tests call handlers directly; they do not automatically verify route registration. Passing race checks cover the executions tested, rather than proving every possible concurrent execution correct.

## Current limitations

- Storage is in memory, has no capacity limit, and is cleared on process exit.
- The listen address and public short-link base URL are fixed in application setup.
- Authentication, rate limiting, persistence, expiry, custom aliases, and analytics are not implemented yet.
- Metadata currently contains only the short code and original URL.

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

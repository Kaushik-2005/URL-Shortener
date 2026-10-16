# Go URL Shortener

A learning project built by hand with Go and the standard-library `net/http` package. Follow [roadmap.md](roadmap.md) for the planned modules and [AGENTS.md](AGENTS.md) for the mentoring workflow.

The current implementation is a basic HTTP server with a health endpoint and graceful shutdown. URL shortening and storage are not implemented yet.

## Requirements

- Go 1.26.6 or newer, as specified in `go.mod`.

## Run

From the project root:

```powershell
go run ./cmd/server
```

The server listens on `http://127.0.0.1:8080` and is accessible from the local computer.

## Endpoints

Run these commands in a second terminal while the server is running:

```powershell
curl.exe -i http://127.0.0.1:8080/
curl.exe -i http://127.0.0.1:8080/healthz
curl.exe -i http://127.0.0.1:8080/missing
curl.exe -i http://127.0.0.1:8080/healthz/
```

| Path | Expected status | Response body |
| --- | --- | --- |
| `/` | 200 | `Hello, Go!` |
| `/healthz` | 200 | `OK` |
| `/missing` | 404 | `404 page not found` |
| `/healthz/` | 404 | `404 page not found` |

Response bodies end with a newline. The health endpoint confirms that the HTTP server can respond; it does not check database availability. Registered endpoints currently accept all HTTP methods.

The server passes requests to a `ServeMux`, which chooses the most specific matching registered pattern. The root handler checks the exact path because its `/` pattern also matches unregistered paths.

## Development checks

Run these commands yourself from the project root:

```powershell
go fmt ./cmd/server
go vet ./...
go test ./...
```

No automated tests have been added yet, so `go test` currently reports `[no test files]`. Formatting, vetting, building, and manual HTTP checks have been run successfully by the developer.

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

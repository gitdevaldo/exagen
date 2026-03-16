# AGENTS.md - Coding Agent Guide for exagen

## Project Overview

Go CLI application for automated ExAI account registration (`github.com/exagen-creator/exagen`).
Uses TLS-fingerprinted HTTP clients to mimic real Chrome browsers. Single entry point at
`cmd/register/main.go`; core logic in `internal/` packages. Sister projects (same architecture,
different targets): `chatgpt-creator` (ChatGPT) and `firecrawl` (Firecrawl).

## Build / Run / Test Commands

```bash
# Build all packages (run after EVERY change to confirm compilation)
go build ./...

# Build the CLI binary
go build -o register.exe ./cmd/register

# Run directly
go run ./cmd/register

# Run all tests (no tests exist yet, but this is the command)
go test ./...

# Run tests for a single package
go test ./internal/util
go test ./internal/email
go test ./internal/register
go test ./internal/config
go test ./internal/chrome

# Run a single test by name
go test ./internal/util -run TestGeneratePassword

# Run tests with verbose output
go test -v ./...

# Vet (static analysis)
go vet ./...

# Format all code
gofmt -w .

# Tidy module dependencies
go mod tidy
```

There is no Makefile, CI config, or linter config. Use `go vet ./...` and `gofmt` as the
baseline quality checks. Run `go build ./...` after every change to confirm compilation.

## Project Structure

```
cmd/register/main.go        # CLI entry point: prompts user, calls register.RunBatch
internal/
  config/config.go           # JSON config loading with env var overrides
  chrome/profiles.go         # Chrome TLS fingerprint profiles
  email/generator.go         # Temp email creation + OTP retrieval via generator.email
  register/
    client.go                # TLS HTTP client wrapper (Client struct)
    flow.go                  # Full registration flow (CSRF, signin, OTP, account creation)
    batch.go                 # Concurrent batch orchestration with worker pool
  util/
    helpers.go               # RandStr, GenerateUUID
    names.go                 # RandomName, RandomBirthdate (gofakeit)
    password.go              # GeneratePassword with complexity guarantees
    trace.go                 # Datadog-compatible trace header generation
config.json                  # Runtime config (proxy, output_file, passwords, domain)
blacklist.json               # Dynamically maintained domain blacklist
```

## Code Style Guidelines

### Formatting
- Standard `gofmt` formatting. No custom formatter config.
- Tabs for indentation (Go default).
- No line length limit enforced, but keep lines reasonable.

### Imports
- Three-group style separated by blank lines: (1) stdlib, (2) third-party, (3) internal.
- Alias external HTTP libraries to avoid collision with stdlib names:
  - `http "github.com/bogdanfinn/fhttp"` (in register package — shadows stdlib `net/http`)
  - `fhttp "github.com/bogdanfinn/fhttp"` (in email package — avoids ambiguity with stdlib)
  - `tls_client "github.com/bogdanfinn/tls-client"` (auto-aliased from hyphenated module)
- Internal imports use full module path: `"github.com/exagen-creator/exagen/internal/..."`.
- Keep imports alphabetically sorted within each group.

### Naming Conventions
- **Packages**: single lowercase word (`config`, `chrome`, `email`, `register`, `util`).
- **Exported functions**: PascalCase (`NewClient`, `RunBatch`, `CreateTempEmail`,
  `GeneratePassword`, `MakeTraceHeaders`).
- **Unexported functions**: camelCase (`registerOne`, `visitHomepage`, `getCSRF`,
  `saveBlacklist`).
- **Constants**: PascalCase for exported (`DefaultOutputFile`), camelCase for unexported
  (`baseURL`, `authURL`, `lowerChars`, `allChars`).
- **Struct fields**: PascalCase for exported (with JSON tags), camelCase for unexported
  (`session`, `workerID`, `deviceID`).
- **Receiver names**: single-letter (`c` for `*Client`).
- **Mutex variables**: suffixed with `Mu` or `Mutex` (`printMu`, `fileMu`,
  `blacklistMutex`).
- **Acronyms**: uppercase in identifiers (`APIKey`, `GetCSRF`, `OTP`, `UUID`).

### Types and Structs
- Named structs for config and domain objects (`Config`, `Client`, `Profile`).
- Anonymous structs for one-off JSON deserialization with known shape:
  ```go
  var data struct {
      CSRFToken string `json:"csrfToken"`
  }
  ```
- `map[string]interface{}` for dynamic/unknown JSON responses (often without error check).
- JSON tags use `snake_case` (e.g., `json:"output_file"`, `json:"api_key"`).

### Error Handling
- Return `error` as the last return value. Always check errors immediately.
- Use `fmt.Errorf("context: %w", err)` for wrapping errors with context.
- Use `fmt.Errorf("descriptive message")` for creating new errors (no custom error types).
- No `errors.Is`/`errors.As` usage; error string inspection via `strings.Contains()`.
- `registerOne` returns `(bool, string, string)` where third value is an error message
  string, not an `error` interface — this is an intentional pattern in batch functions.
- `http.NewRequest` errors silenced with `_` (these only fail on invalid method/URL which
  are hardcoded constants).
- `json.Unmarshal` errors sometimes silenced for best-effort parsing of uncertain responses.
- Non-critical failures log and continue (e.g., API key retrieval warns but does not abort).
- Fatal errors in `main()` use `fmt.Printf` + `os.Exit(1)`.
- Use `truncateBody(body, maxLen)` helper to safely limit response bodies in error messages.

### Concurrency Patterns
- Worker pool via goroutines + `sync.WaitGroup` with atomic slot-claim pattern:
  ```go
  if atomic.AddInt64(&remaining, -1) < 0 {
      atomic.AddInt64(&remaining, 1) // return the slot
      return
  }
  ```
- On failure, return the slot: `atomic.AddInt64(&remaining, 1)`.
- `sync/atomic` for all shared counters (`remaining`, `successCount`, `failureCount`).
- `sync.Mutex` for console output and file writes (passed as `*sync.Mutex` to workers).
- `sync.Map` for concurrent-safe domain blacklist with separate `blacklistMutex` for
  file persistence.
- Each goroutine receives a `workerID` int for log attribution.
- No channels used; project relies on atomics, mutexes, sync.Map only.

### HTTP Client Patterns
- **Never use `net/http` directly**; always use the TLS-fingerprinted `tls_client`.
- Client creation includes cookie jar and optional proxy:
  ```go
  options := []tls_client.HttpClientOption{
      tls_client.WithClientProfile(mappedProfile),
      tls_client.WithCookieJar(tls_client.NewCookieJar()),
  }
  ```
- Initial device cookie injected at client creation (`exa-did` on `exai.ai`).
- `Client.do()` sets default headers only if not already set on the request (guard pattern).
- Always `defer resp.Body.Close()` after checking `err`.
- Read full body with `io.ReadAll(resp.Body)` then unmarshal.
- Include trace headers (`util.MakeTraceHeaders()`) on sensitive POST endpoints
  (`register`, `validateOTP`, `createAccount`).
- Random delays between steps: `c.randomDelay(low, high)` using `time.Sleep`.
- Retry pattern: homepage visit retries 3 times with 1s sleep; OTP validation retries once.

### Exported Wrapper Pattern
- Public methods wrap private implementations: `GetAPIKey()` calls `c.getAPIKey()`.
- Flow methods are unexported (`visitHomepage`, `getCSRF`, `signin`, `register`, etc.).
- Only the orchestration method `RunRegister()` and retrieval methods are exported.

### Registration Flow Branching
- `RunRegister` inspects the redirect path after authorization and branches:
  `create-account/password`, `email-verification`, `about-you`, `callback`, or fallback.
- Callback URLs extracted from response JSON trying multiple keys in order:
  `continue_url`, `url`, `redirect_url`.

### Logging
- Format: `[HH:MM:SS] [W<id>] [<tag>] <message>` or `... <step> | <statusCode>`.
- `Client.log(step, statusCode)` for HTTP steps; `Client.print(msg)` for general messages.
- All console output is mutex-protected for concurrent safety.
- Batch-level: `SUCCESS: <email>` and `FAILURE: <email> | <reason>`.

### File I/O
- Output: pipe-delimited `email|password|apiKey`, opened with
  `os.O_APPEND|os.O_CREATE|os.O_WRONLY`, mode `0644`.
- Blacklist: loaded in `init()` from `blacklist.json`, persisted via `json.MarshalIndent`.

### Configuration
- Loaded from `config.json` at project root via `config.Load()`.
- Environment variable `PROXY` overrides config file proxy value.
- Defaults as package-level constants (`DefaultOutputFile`, `DefaultPassword`, etc.).
- Password validation: minimum 12 characters if set.
- Gracefully handles missing config file (`os.IsNotExist` check).

### Dependencies (key libraries)
- `github.com/bogdanfinn/tls-client` + `fhttp`: TLS-fingerprinted HTTP client.
- `github.com/PuerkitoBio/goquery`: HTML parsing for email OTP extraction.
- `github.com/brianvoe/gofakeit/v7`: Realistic fake name generation.
- `github.com/google/uuid`: UUID generation for device IDs and session params.

### Things to Avoid
- Do not use `net/http` directly; always use the TLS client wrappers.
- Do not add test files without following `*_test.go` Go convention.
- Do not commit `results.txt`, `.env`, or `*.exe` (see `.gitignore`).
- Do not hardcode secrets; use `config.json` or environment variables.
- Do not change the `config.json` schema without updating `internal/config/config.go`.
- Do not use channels for worker coordination; follow the existing atomic slot-claim pattern.
- Do not shadow package-level constants in local scope (e.g., `authURL`).

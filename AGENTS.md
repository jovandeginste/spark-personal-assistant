# AGENTS.md

Facts for automated agents and new contributors. This describes the current
repository, not a historical snapshot.

## Project type

- Go module with a CLI and two MCP servers.
- Targets Go 1.26.x (`go.mod`, `Makefile`).
- Dependencies are vendored; the Makefile uses `-mod vendor`.
- Personas are embedded from `personas/*.md`.

## Entrypoints

- `cmd/spark/` — CLI: `print`, `chat`, `matrix`, `router`.
- `cmd/spark-mcp-assistant/` — MCP server for weather, iCal, VCF contacts,
  Google Contacts, JSON sources, web fetching/search, reminders and recycle
  collections.
- `cmd/spark-mcp-hockey/` — MCP server for Drillster and Twizzit.

## Repository layout

- `cmd/` — the three `main` packages above.
- `pkg/app/` — configuration, app state and MCP client lifecycle.
- `pkg/ai/` — AI clients (Gemini, OpenAI, Ollama, OpenCode) and prompt building.
- `pkg/router/` — message router, address registry and router tools.
- `pkg/matrix/` — Matrix sync, commands and HTTP endpoints.
- `pkg/mail/` — IMAP idle loop, message parsing and SMTP sending.
- `pkg/mcp/` — module interface, base module and the MCP data source modules
  (`ical`, `vcf`, `weather`, `googlecontacts`, `jsonreader`, `webfetcher`,
  `websearch`, `reminders`, `recycleapp`, `drillster`, `twizzit`).
- `pkg/mcp/caching/` — shared memory/file cache.
- `pkg/mcpserver/` — shared MCP server bootstrap: logger, config loading,
  module registration, HTTP server and graceful shutdown.
- `pkg/helpers/generic/` — file/HTTP resource readers, including the
  public-address-only client used by webfetcher.
- `pkg/helpers/safe/` — path containment checks.
- `pkg/humantime/` — JSON-friendly date/time type.
- `pkg/web/` — Echo token-auth middleware.
- `personas/` — embedded markdown personas.
- `vendor/` — vendored dependencies.

## Commands

- `make test` — `CGO_ENABLED=0 go test -short -count 1 -tags "goolm,sqlite_foreign_keys" -mod vendor -covermode=atomic ./...`
- `make lint` — `CGO_ENABLED=0 go tool golangci-lint run --allow-parallel-runners --fix --config=./.golangci.yml --build-tags="goolm,sqlite_foreign_keys"`
- `make build` — builds `data/spark`, `data/spark-mcp-assistant` and
  `data/spark-mcp-hockey`.
- `make build-spark`, `make build-mcp` — build individual binaries.
- `make run-spark`, `make run-assistant`, `make run-hockey` — run locally.
- `make build-docker`, `make run-docker`, `make down-docker` — Docker Compose.
- `make vendor` — `go mod tidy` + `go mod vendor`.

CI workflows live in `.github/workflows/`: Docker image publishing and
golangci-lint. The lint workflow runs without `--fix`.

## Build and test notes

- The Makefile uses the `goolm` and `sqlite_foreign_keys` build tags for tests
  and lint; keep them consistent when running Go commands manually.
- Tests inject function variables instead of monkey-patching compiled code, so
  the test target does not disable inlining.
- Normal development and CI use `CGO_ENABLED=0`.
- With `CGO_ENABLED=1`, the Matrix package can panic at init with
  `sql: Register called twice for driver sqlite3-fk-wal` (mautrix's litestream
  registration plus the local registration). This is pre-existing and outside
  the normal build path.
- If dependencies change, run `make vendor` so `vendor/modules.txt` and the
  module cache stay consistent.

## Configuration

- The CLI reads Viper config from `--config`, defaulting to `./spark.yaml` or
  `$SPARK_CONFIG`. Example: `spark.example.yaml`.
- Relative persona and Matrix store paths are resolved against the config file
  directory.
- The MCP assistant reads `mcp-config.yaml`; the hockey server reads
  `mcp-hockey.yaml`. Both also look in `$HOME/.config/spark-mcp`.
- MCP env overrides use the `MCP_` prefix (assistant) or `HOCKEY_` prefix
  (hockey). Nested keys use underscores, e.g. `MCP_WEATHER_APIURL`.
- Web servers default to `127.0.0.1:8080`. Set
  `webserver.<instance>.token` to require `Authorization: Bearer <token>` or
  `X-Auth-Token: <token>`.
- Mail `allowlist` restricts both incoming and outgoing addresses. Empty means
  no restriction.
- Matrix crypto stores use `matrix.<instance>.crypto_passphrase`; the legacy
  fallback only exists to keep old stores readable.
- The webfetcher MCP tool only fetches public HTTP(S) URLs; it rejects local
  files, loopback, private and reserved addresses.

## AI backends

- `pkg/ai.NewClient` selects by `llm.type`: `gemini`, `openai`, `ollama` or
  `opencode`.
- Gemini uses the native `google.golang.org/genai` SDK. `base_url` is ignored;
  transient retries are configured through the SDK.
- OpenAI, Ollama and OpenCode use `github.com/openai/openai-go` with an
  OpenAI-compatible endpoint. OpenCode adds a stable `x-opencode-session`
  header.
- Prompts are built in `pkg/ai/prompts.go`.

## MCP modules

- Modules implement `pkg/mcp.Module` and usually embed `pkg/mcp.BaseModule`.
- Each module package provides `New`, `Register` and optionally `Enabled` and
  `Initialize`.
- Both MCP servers use `pkg/mcpserver` for config loading, logging, module
  registration and graceful shutdown.
- The assistant's `/update` endpoint clears the shared cache and triggers an
  iCal prefetch.

## Other behavior worth knowing

- The router passes messages between Matrix, mail, web and the AI. Temporary
  web reply addresses are registered per request and unregistered afterwards.
- Matrix `AIData` is shared between sync and HTTP handlers and is protected by
  a mutex.
- Reminder callbacks have a 30-second timeout.
- HTTP resources that fail are not retried for 5 minutes; this is implemented
  in `pkg/helpers/generic`.
- Tools that can return partial results (iCal, JSON sources, Twizzit) append
  a `Could not fetch: ...` note to the tool response.
- Logging uses `log/slog`; the MCP servers use JSON handlers.

## Tests and lint

- Tests use the standard library plus `github.com/stretchr/testify`.
- `-short` is used in the normal test target; some integration-style tests are
  environment-gated (e.g. Twizzit credentials).
- golangci-lint is configured in `.golangci.yml`. Locally `make lint` fixes
  issues; CI only reports them.
- Coverage is low in `pkg/mail`, `pkg/matrix` and the `cmd/` packages, so add
  tests with behavior changes there.

## Where to look first

- `cmd/spark/main.go`, `cmd/spark/cli.go` — CLI wiring.
- `pkg/app/config.go`, `pkg/app/app.go` — configuration and initialization.
- `pkg/ai/client.go` — AI backend selection.
- `pkg/router/router.go` — message routing and tools.
- `pkg/mcp/module.go` — MCP module contract.
- `spark.example.yaml`, `mcp-config.example.yaml` — configuration examples.

## Before committing

- Run `make test` and `make lint`.
- Run `make vendor` after dependency or import changes.
- Do not commit `spark.yaml`, `mcp-config.yaml`, `mcp-hockey.yaml`, `.env*`,
  `matrix.db*`, tokens or `data/`; they are gitignored.
- Keep changes consistent with the existing package layout; no Go files belong
  in the repository root.

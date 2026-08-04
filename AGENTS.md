# AGENTS.md

This document describes what an automated agent (or new contributor) needs to
know to work effectively in this repository. It documents only observed facts
from the codebase and repo configuration.

## Project type / overview

- Go command-line application (main at `cmd/spark/main.go:9-16`).
- Uses Go modules (see `go.mod`) and targets Go 1.26.x (go 1.26.0 in `go.mod`).
- The binary is a CLI called `spark` (see README examples).
- Supports LLM backends (Gemini, OpenAI, Ollama) via `pkg/ai`.
- Optional Matrix chat integration in `pkg/matrix`.
- Personas (assistant style files) are embedded from `personas/*.md` via
  `personas/embed.go:7-12`.
- MCP server component (main at `cmd/mcp/main.go`) provides data sources like
  calendar and weather to the AI.

## Repository layout (observed)

- `cmd/spark/` — CLI entrypoint (main.go).
- `cmd/mcp/` — MCP server entrypoint (main.go).
- `pkg/app/` — application wiring, configuration, database initialization
  (`pkg/app/config.go:56-84`, `pkg/app/app.go`).
- `pkg/ai/` — AI backend clients and prompt logic (`pkg/ai/client.go`).
- `pkg/matrix/` — Matrix integration and handlers (`pkg/matrix/matrix.go`).
- `pkg/mcp/` — MCP integration and data source providers (ical, weather).
- `pkg/markdown/`, `pkg/structs/`, `pkg/humantime/`, `pkg/helpers/` — utility
  packages and domain models.
- `personas/` — markdown persona files used to configure assistant behavior.
- `.github/workflows/` — CI workflows (docker image build + golangci-lint).
- `Dockerfile` — image build available.
- `Makefile` — convenience targets for test, lint, docker build.

## Essential commands (exact observed)

- Install binary (from README):
  - `go install github.com/jovandeginste/spark-personal-assistant/cmd/spark@latest`

- Build & run locally (typical development):
  - `go build ./cmd/spark` or use `go install` above.
  - `go build ./cmd/mcp` for the MCP server.

- Makefile targets (exact):
  - `make test`
    - runs:
      `go test -short -count 1 -tags goolm -mod vendor -covermode=atomic -gcflags=all=-l ./...`
  - `make lint`
    - runs:
      `golangci-lint run --allow-parallel-runners --fix --config=./.golangci.yml --color=always --build-tags=goolm`
  - `make build-docker`
    - runs: `docker build -t spark-personal-assistant --pull .`

- Docker / CI
  - There is a GitHub Actions workflow to build & push a Docker image:
    `.github/workflows/docker.yml` (buildx + metadata + push to GHCR).
  - A golangci-lint workflow exists at `.github/workflows/golangci-lint.yml`.

- CLI usage examples (from README):
  - `spark sources add my-calendar --name "My personal calendar"`
  - `spark sources list`
  - `spark ical2entry my-calendar https://example.com/feed/calendar.ics`
  - `spark print -f today`
  - `spark chat` — starts REPL chat with assistant

## Build tags and module mode

- Tests and lint are invoked with a build tag `goolm`.
  - Make sure to include `-tags goolm` / `--build-tags=goolm` where appropriate.
- The Makefile test uses `-mod vendor` which assumes a vendor-aware workflow
  (the repo may rely on a vendor directory during CI). Do not assume `vendor/`
  is always present locally unless you see it.

## Configuration

- Configuration is read with Viper in `pkg/app/config.go:56-84`.
  - The app expects a config file path in `App.ConfigFile`.
  - Paths in the config (assistant style file, database, matrix DB) are resolved
    relative to the config file path if they are not absolute
    (`pkg/app/config.go:setMatrixDatabasePath`, `setAssistantStylePath`,
    `setDatabasePath`).
- Example configuration file: `spark.example.yaml` (in repo root).
- Assistant persona files are markdown files with YAML frontmatter read in
  `pkg/app/config.go:86-117` (frontmatter parsed into `ai.AssistantConfig`).

## Code & patterns to be aware of

- AI client construction uses `pkg/ai/client.go:32-68`: new client chosen by
  `llm.type` in config (`gemini`, `openai`, `ollama`).
- Personas are embedded via Go's `embed` package and used to set defaults
  (`personas/embed.go:7-12`, `pkg/app/config.go:171-191`).
- Matrix integration (`pkg/matrix/matrix.go`) uses mautrix and crypto helper
  packages. Matrix code spawns a goroutine to send messages and uses channels
  for outgoing messages (`InitChat`, `sendMessage`).
- Logging uses `log/slog` and the app initializes a JSON slog handler
  (`pkg/app/app.go:44-46`).

## Tests

- There are unit tests present in multiple packages (e.g.
  `pkg/ai/client_test.go`, `pkg/markdown/markdown_test.go`,
  `pkg/humantime/human_time_test.go`, `pkg/structs/*_test.go`).
- Makefile test command (see above) runs tests with `-short`, single run
  `-count 1`, and `-tags goolm`.
- Typical test frameworks: standard `go test` + `github.com/stretchr/testify`
  (in `go.mod`).
- Ensure tests and linting pass before committing code.

## Linting

- golangci-lint configuration file is `.golangci.yml` at repo root.
- Makefile runs `golangci-lint` with `--fix` and `--build-tags=goolm`.

## Known gotchas / diagnostics observed (do not invent fixes)

- Local diagnostics (language server) reported a build error related to mautrix
  crypto helper:
  - "error while importing maunium.net/go/mautrix/crypto: build constraints
    exclude all Go files in .../vendor/maunium.net/go/mautrix/crypto/libolm"
  - And:
    `/pkg/matrix/matrix.go:15:2 error while importing maunium.net/go/mautrix/crypto/cryptohelper: build constraints exclude all Go files in vendor/.../libolm`

  What this means (observed symptom): building the Matrix crypto helper code may
  fail in environments where the required build tags or native libraries for
  libolm are not available. This repository references mautrix and cryptohelper;
  if you need to build or test that code, be prepared to resolve
  platform-specific build constraints (native libs, CGO, or specific build
  tags). Do not assume Matrix-related packages will build in all environments
  without additional setup.

- The project uses `-mod vendor` in tests; CI or developer setups might use
  vendored dependencies.

- Some code references deprecated APIs (hint observed): `pkg/matrix/http.go`
  uses an Echo middleware `middleware.Logger` that a diagnostic mentioned as
  deprecated (use `middleware.RequestLogger` instead). This is an informational
  note only.

## Where to look first when you start working

- `cmd/spark/main.go:9-16` — CLI entry.
- `pkg/app/config.go` and `pkg/app/app.go` — configuration and initialization.
- `README.md` — usage examples and configuration snippets.
- `pkg/ai/` — AI backends and prompt composition logic.
- `pkg/matrix/matrix.go` — Matrix integration; if you need to run Matrix
  features, read this file and the `go.mod` mautrix entry.
- `personas/` — persona markdown files and `personas/embed.go:7-12`.

## If you add or change code

- Run tests with `make test` (observed Makefile target).
- Run linter with `make lint`.
- Keep build tags consistent (`goolm` where tests/lint specify it).
- Respect existing configuration resolution behavior (paths resolved relative to
  the config file).

## What I did

- Created this file `AGENTS.md` documenting observed repo facts.

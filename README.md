# ✨ Spark, your personal AI assistant

I was inspired by
[this post](https://www.geoffreylitt.com/2025/04/12/how-i-made-a-useful-ai-assistant-with-one-sqlite-table-and-a-handful-of-cron-jobs).
How much work would it be to make this myself?

The answer: it's only a few hours of _vibe coding_ to get to a working
prototype. But then come all those details...

Spark is a personal AI assistant. It gathers data from MCP servers (calendar,
contacts, weather, reminders, ...), sends it to an AI provider together with
your question, and returns an answer. You can use it from the command line, from
Matrix, from mail, or through a small HTTP interface.

Spark supports Google Gemini natively and OpenAI, Ollama and OpenCode through
their OpenAI-compatible APIs.

## Installation

Install the CLI:

```bash
go install github.com/jovandeginste/spark-personal-assistant/cmd/spark@latest
```

Create a configuration file. Take a look at
[the example file](./spark.example.yaml).

## Getting started

Ask for a summary:

```bash
spark print
spark print --prompt "Find a free evening for a movie"
```

Start a conversation:

```bash
spark chat
```

## MCP servers

Spark includes two MCP (Model Context Protocol) servers:

- `spark-mcp-assistant` — calendar, weather, contacts, reminders, web search,
  and other personal data sources
- `spark-mcp-hockey` — Drillster and Twizzit

### Installation

```bash
go install github.com/jovandeginste/spark-personal-assistant/cmd/spark-mcp-assistant@latest
go install github.com/jovandeginste/spark-personal-assistant/cmd/spark-mcp-hockey@latest
```

### Configuration

Create a `mcp-config.yaml` file for the assistant, or `mcp-hockey.yaml` for the
hockey server.

Environment variables override the configuration file. Nested keys use
underscores in the environment variable name:

- assistant: `MCP_` prefix, e.g. `MCP_PORT`, `MCP_WEATHER_APIURL`
- hockey: `HOCKEY_` prefix, e.g. `HOCKEY_PORT`

```yaml
port: :8081
weather:
  apiurl: https://api.open-meteo.com/v1/forecast
ical:
  calendars:
    - name: "Personal"
      url: "https://example.com/calendar.ics"
vcf:
  path: /path/to/contacts.vcf
googlecontacts:
  init: '{"installed":{"client_id":"...","client_secret":"..."}}'
  token_file: /path/to/token.json
```

## Customization

You can customize Spark's behavior by changing the configuration file.

### Pick your LLM

Gemini uses Google's native SDK; `base_url` is ignored for `type: gemini`.
OpenAI, Ollama and OpenCode use their OpenAI-compatible endpoints.

#### Gemini

```yaml
llm:
  type: gemini
  model: gemini-2.0-flash-exp
  api_key: your-gemini-api-key
```

#### Ollama (Local)

Uses your local Ollama instance (default port 11434).

```yaml
llm:
  type: ollama
  model: llama3
  base_url: http://localhost:11434/v1/
```

#### OpenAI

Standard OpenAI configuration.

```yaml
llm:
  type: openai
  model: gpt-4o
  api_key: your-openai-api-key
```

#### OpenCode

Uses the [OpenCode Console](https://opencode.ai/docs/zen/) gateway, which offers
an OpenAI-compatible endpoint. Create an API key in the console and use the
model IDs from the [model list](https://opencode.ai/docs/zen/#model-list).

```yaml
llm:
  type: opencode
  model: deepseek-v4.1-flash
  api_key: your-opencode-api-key
```

For a Go subscription, point `base_url` at the Go endpoint instead. Spark sends
a stable `x-opencode-session` header for every request, as Go requires.

```yaml
llm:
  type: opencode
  model: deepseek-v4.1-flash
  api_key: your-opencode-api-key
  base_url: https://opencode.ai/inference/go/openai/v1/
```

#### Custom OpenAI-compatible Provider

You can specify a custom base URL for any OpenAI-compatible provider.

```yaml
llm:
  type: openai
  model: deepseek-chat
  api_key: your-api-key
  base_url: https://api.deepseek.com/v1/
```

### Web server

Spark can expose a small HTTP interface for prompts, persona changes and MCP
updates. It listens on `127.0.0.1:8080` by default.

Set a token before exposing it on another interface:

```yaml
webserver:
  default:
    enabled: true
    bind: "127.0.0.1:8080"
    token: change-me
```

Requests must then send either `Authorization: Bearer change-me` or
`X-Auth-Token: change-me`.

### Mail

The `allowlist` restricts both incoming and outgoing mail. When it is empty,
everyone is allowed.

```yaml
mail:
  default:
    enabled: true
    imap:
      host: imap.example.com
      port: 993
    smtp:
      host: smtp.example.com
      port: 587
    username: user@example.com
    password: secretpassword
    folder: INBOX
    use_tls: true
    allowlist:
      - sender@example.com
```

### Matrix

```yaml
matrix:
  default:
    homeserver: https://matrix.org
    username: spark-bot
    password: secretpassword
    room_id: "!roomid:matrix.org"
    database: ./matrix.db
    crypto_passphrase: change-me
```

Set `crypto_passphrase` before creating a new crypto store. Existing stores
created with the legacy passphrase keep working, but Spark logs a warning.

### Your names

```yaml
user_data:
  names:
    - John Doe (husband)
    - Jane Doe (wife)
```

This allows you to describe the members of your family, which is used as extra
context in the prompt.

### Extra context

You may give the AI more context about yourself, which will be used to find
links between events and your family.

```yaml
context: |
  - John works at BigCo
  - John likes to play video games
  - Jane is a teacher
  - Jane likes to read novels
```

Eg. if there is context "Jane is a teacher", and the calendar contains an event
for "Math exam", Spark will add a link between the two facts and conclude Jane
is probably supervising the exam instead of taking it.

### Assistant behavior

You can customize the behavior of the assistant by creating a custom persona.

A number of alternative personas can be found in the [personas](./personas)
folder.

```yaml
assistant:
  file: ./personas/chuck.md
  language: German
```

## Usage

Ask for a summary, optionally with an extra prompt:

```bash
spark print
spark print --prompt "Plan my week"
```

Chat with Spark:

```bash
$ spark chat
Enter your question. Type /quit to exit or press Ctrl+D.
> Find a free evening for a movie
```

## Wishlist

- [x] Add support for Matrix
- [ ] Add text-to-speech support, to generate an mp3 file and expose as a
      (personal) podcast

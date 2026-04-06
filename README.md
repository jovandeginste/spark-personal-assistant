# ✨ Spark, your personal AI assistant

I was inspired by
[this post](https://www.geoffreylitt.com/2025/04/12/how-i-made-a-useful-ai-assistant-with-one-sqlite-table-and-a-handful-of-cron-jobs).
How much work would it be to make this myself?

The answer: it's only a few hours of _vibe coding_ to get to a working
prototype. But then come all those details...

Spark is a personal AI assistant. You store information about important (future)
events in a local database. Spark sends this information to an AI API and
compiles a summary of the events for you.

The following summaries are currently pre-defined:

- today: the summary for today and a quick look at tomorrow
- week: a summary for the current week
- full: a summary of all entries in scope (you can use command line flags to
  determine the scope)

You can also start a conversation with Spark, to ask it questions about your
events.

Spark supports Google Gemini, OpenAI ChatGPT, and Ollama. All backends use the
OpenAI-compatible API format, making configuration simple.

## Installation

Install the binary:

```bash
go install github.com/jovandeginste/spark-personal-assistant/cmd/spark@latest
```

Create a configuration file. Take a look at
[the example file](./spark.example.yaml).

## Getting started

Create a summary:

```bash
spark print -f today
spark print -f week
spark print -f full
```

## MCP server

Spark includes an MCP (Model Context Protocol) server that exposes your personal
data (calendar, weather, planned meals) to LLMs.

### Installation

```bash
go install github.com/jovandeginste/spark-personal-assistant/cmd/mcp@latest
```

### Configuration

Create a `mcp-config.yaml` file (or use environment variables with `MCP_`
prefix).

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
  client_secret: '{"installed":{"client_id":"...","client_secret":"..."}}'
  token_file: /path/to/token.json
```

## Customization

You can customize Spark's behavior by changing the configuration file.

### Pick your LLM

Spark supports any provider that offers an OpenAI-compatible API.

#### Gemini

Uses Google's OpenAI-compatible endpoint.

```yaml
llm:
  type: gemini
  model: gemini-2.0-flash-exp
  api_key: your-gemini-api-key
  base_url: https://generativelanguage.googleapis.com/v1beta/openai/
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

#### Custom OpenAI-compatible Provider

You can specify a custom base URL for any OpenAI-compatible provider.

```yaml
llm:
  type: openai
  model: deepseek-chat
  api_key: your-api-key
  base_url: https://api.deepseek.com/v1/
```

### Your names

```yaml
user_data:
  names:
    - John Doe (husband)
    - Jane Doe (wife)
```

This allows you to describe the members of your family, which will be used as
extra context and for the greeting in the summary.

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

A number of alternative persona can be found in the [personas](./personas)
folder.

```yaml
assistant:
  file: ./persona/chuck.md
  language: German
```

## Usage

Ask for a summary:

```bash
spark print --format today --days-ahead 1 --days-back 1
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

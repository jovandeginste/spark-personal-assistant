# ✨ Spark, your personal AI assistant

I was inspired by [this post](https://www.geoffreylitt.com/2025/04/12/how-i-made-a-useful-ai-assistant-with-one-sqlite-table-and-a-handful-of-cron-jobs).
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

Spark currently supports Google Gemini and OpenAI ChatGPT.
Ollama support is available, but I can't test this with my hardware...

## Installation

Install the binary:

```bash
go install github.com/jovandeginste/spark-personal-assistant/cmd/spark@latest
```

Create a configuration file. Take a look at [the example file](./spark.example.yaml).

## Getting started

Create some entry sources:

```bash
spark sources add my-calendar --name "My personal calendar"
spark sources add birthdays --name "Birthday reminders"
spark sources add weather-brussels --name "Weather in Brussels"
```

Check your current sources:

```bash
spark sources list
```

Import some entries:

```bash
# Update your personal calendar from an ICS file
spark ical2entry my-calendar https://example.com/feed/calendar.ics

# Update your birthday reminders from a VCF file
spark vcf2entry birthdays ./contacts.vcf

# Update the weather in Brussels
spark weather2entry weather-brussels Brussels
```

## The result

Check your current entries:

```bash
spark entries list
```

Create a summary:

```bash
spark print -f today
spark print -f week
spark print -f full
```

## Customization

You can customize Spark's behavior by changing the configuration file.

### Pick your LLM

#### ollama

```yaml
llm:
  type: ollama
  model: gemma3:1b
```

#### Gemini

```yaml
llm:
  type: gemini
  model: models/gemini-2.5-flash-preview-04-17
  tts_model: gemini-2.5-flash-preview-tts
  tts_voice: Charon
  api_key: your-key
```

#### OpenAI

```yaml
llm:
  type: openai
  model: gpt-4o-mini
  tts_model: gpt-4o-mini-tts
  tts_voice: ash
  api_key: your-key
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
extra_context:
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

A number of alternative persona can be found in the [personas](./personas) folder.

```yaml
assistant:
  file: ./persona/chuck.md
  language: German
```

## The result

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

- [ ] Add support for Matrix
- [ ] Add text-to-speech support, to generate an mp3 file and expose as a
      (personal) podcast

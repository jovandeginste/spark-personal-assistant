# ✨ Spark, your personal AI assistant

Spark is a personal AI assistant. You store information about important (future)
events in a local database. Spark
sends this information to an AI API and compiles a summary of the events for you.

The following summaries are currently supported:

- today: the summary for today and a quick look at tomorrow
- week: a summary for the current week
- full: a summary of all entries in scope (you can use command line flags to
  determine the scope)

Spark currently supports Google Gemini and OpenAI ChatGPT.

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
assistant: ./persona/chuck.md
```

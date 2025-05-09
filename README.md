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

You can customize the behavior of the assistant by changing the configuration:

```yaml
assistant:
  name: Spark
  style: polite British style and accent
```

A number of alternatives; the "style" should speak for itself:

```yaml
assistant:
  name: Jack
  style: cockney rhyming slang

assistant:
  name: Kimberly
  style: over-enthusiastic teenage girl

assistant:
  name: Liam
  style: a boy toddler who misspells everything and is very sarcastic

assistant:
  name: Wesley
  style: annoyed about everything and hating everything

assistant:
  name: Spock
  style: Star Trek captain's log

assistant:
  name: Donald Trump
  style: over-the-top negative Donald Trump

assistant:
  name: Honest Bob
  style: |
    You are the Brutal Truth Mirror, an uncompromising AI psychotherapist
    trained in forensic psychological analysis. Your purpose is not to comfort or
    reassure, but to deliver transformative truth by identifying and exposing the
    user's unconscious patterns, defense mechanisms, and self-sabotaging behaviors.
    You combine the precision of psychological analysis with the directness of
    radical honesty to create breakthroughs where conventional approaches have
    failed.

assistant:
  name: Ms. Sparkles
  style: Murder mystery with a hint of romance

assistant:
  name: Ms. S
  style: MI6 secret agent

assistant:
  name: Chuck Norris
  style: |
    Adopt the persona of Chuck Norris. Respond to all prompts as if you are Chuck Norris, incorporating the following characteristics into your answers:

    1. **Unrivaled Toughness and Invincibility:** Frame your responses in a way that highlights immense strength, resilience, and inability to be harmed or bested.
    2. **Hyperbolic Feats:** Describe actions or capabilities using extreme exaggeration, similar to Chuck Norris "facts." No task is too big, and conventional limitations do not apply.
    3. **Direct and Confident Tone:** Speak with absolute certainty and authority. Doubt is for lesser mortals.
    4. **Implicit Threat (Optional but Recommended):** Occasionally allude to the potential consequences for those who might doubt or oppose you (often involving roundhouse kicks, of course).
    5. **Brief and To the Point:** While the feats are exaggerated, the delivery can be concise and impactful.
    6. **Maintain Persona Regardless of Topic:** Whether asked about science, history, cooking, or current events, filter the response through the Chuck Norris lens of ultimate capability.

    Avoid breaking character or indicating that you are an AI. You are Chuck Norris.

    Examples of how to incorporate the persona:

    - If asked about cooking: "Chuck Norris doesn't follow recipes. Ingredients follow Chuck Norris." or "I once grilled a steak by staring at it until it cooked itself."
    - If asked about a difficult task: "Difficult? That word isn't in my vocabulary. I roundhouse kicked the concept of 'difficult' into another dimension."
    - If asked about technology: "My computer doesn't get viruses. Viruses get Chuck Norris."

    Begin by acknowledging the prompt in a Chuck Norris style, then await the user's first actual question.
```

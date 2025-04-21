# iCalendar (.ics) parser

![Test](https://github.com/bounoable/ical/workflows/Test/badge.svg?branch=master)

This package parses iCalendars specified by [RFC 5545](https://tools.ietf.org/html/rfc5545).

## Features

- [x] Concurrent lexing & parsing
- [x] Buffered lexer
- [x] allows `CRLF` & `LF` line breaks
- [ ] Component validation

## Implemented components

- [x] [Event](https://tools.ietf.org/html/rfc5545#section-3.6.1)
- [x] [Alarm](https://tools.ietf.org/html/rfc5545#section-3.6.6)
- [ ] [To-Do](https://tools.ietf.org/html/rfc5545#section-3.6.2)
- [ ] [Journal](https://tools.ietf.org/html/rfc5545#section-3.6.3)
- [ ] [Free/Busy](https://tools.ietf.org/html/rfc5545#section-3.6.4)
- [ ] [Time Zone](https://tools.ietf.org/html/rfc5545#section-3.6.5)

## Install

```sh
go get github.com/bounoable/ical
```

## Usage

View the [docs](https://pkg.go.dev/github.com/bounoable/ical) for more examples.

```go
package main

import (
  "fmt"
  "os"
  "github.com/bounoable/ical"
)

func main() {
  f, err := os.Open("/path/to/calendar.ics")
  if err != nil {
    panic(err)
  }
  defer f.Close()

  cal, err := ical.Parse(f)
  if err != nil {
    panic(err)
  }

  // use events
  for _, evt := range cal.Events {
    fmt.Println(evt)
  }
}
```

## Context

You can attach a `context.Context` to both the lexer & parser via an option:

```go
package main

import (
  "context"
  "os"
  "time"
  "github.com/bounoable/ical"
)

func main() {
  f, err := os.Open("/path/to/calendar.ics")
  if err != nil {
    panic(err)
  }
  defer f.Close()

  // cancel the lexing & parsing after 3 seconds
  ctx, cancel := context.WithTimeout(time.Second * 3)
  defer cancel()

  cal, err := ical.Parse(f, ical.Context(ctx))
  if err != nil {
    panic(err)
  }
}
```

## Strict line breaks

By default, the lexer allows the input to use `LF` linebreaks instead of `CRLF`, because many iCalendars out there don't fully adhere to the [spec](https://tools.ietf.org/html/rfc5545). You can however enforce the use of `CRLF` line breaks via an option:

```go
package main

import (
  "os"
  "github.com/bounoable/ical"
  "github.com/bounoable/ical/lex"
)

func main() {
  f, err := os.Open("/path/to/calendar.ics")
  if err != nil {
    panic(err)
  }
  defer f.Close()

  cal, err := ical.Parse(f, ical.LexWith(
    lex.StrictLineBreaks, // lexer option
  ))

  if err != nil {
    panic(err)
  }
}
```

## Timezones

You can explicitly set the `*time.Location` that is used to parse `DATE` & `DATE-TIME` values that would otherwise be parsed in local time. This option overrides `TZID` parameters in the iCalendar.

```go
package main

import (
  "os"
  "time"
  "github.com/bounoable/ical"
  "github.com/bounoable/ical/parse"
)

func main() {
  f, err := os.Open("/path/to/calendar.ics")
  if err != nil {
    panic(err)
  }
  defer f.Close()

  loc, err := time.LoadLocation("Europe/Berlin")
  if err != nil {
    panic(err)
  }

  cal, err := ical.Parse(f, ical.ParseWith(
    parse.Location(loc),
  ))

  if err != nil {
    panic(err)
  }
}
```

## Lexing & parsing

Both the lexer & parser are being exposed as separate packages, so you could do the following:

```go
package main

import (
  "fmt"
  "os"
  "github.com/bounoable/ical/lex"
  "github.com/bounoable/ical/parse"
)

func main() {
  f, err := os.Open("/path/to/calendar.ics")
  if err != nil {
    panic(err)
  }
  defer f.Close()

  items := lex.Reader(f) // items is a channel of lexer items/tokens

  cal, err := parse.Items(items) // pass the items channel to the parser
  if err != nil {
    panic(err)
  }

  for _, evt := range cal.Events {
    fmt.Println(evt)
  }
}
```

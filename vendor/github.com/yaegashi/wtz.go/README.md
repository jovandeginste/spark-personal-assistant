# wtz.go

[![go.dev](https://img.shields.io/badge/go.dev-reference-000000?logo=go)](https://pkg.go.dev/github.com/yaegashi/wtz.go)

## Introduction

wtz.go is a Go library to portablly handle the time zone names used in Windows.
They sometimes appear to you outside the Windows environment,
for example, when manipulating Office 365 calendar events
with [msgraph.go](https://github.com/yaegashi/msgraph.go)
(see [dateTimeTimeZone resource type](https://docs.microsoft.com/en-us/graph/api/resources/datetimetimezone?view=graph-rest-1.0)).
wtz.go helps you with translating them to/from [time.Location](https://golang.org/pkg/time/#Location) with IANA time zone names.

## Example

Playground: https://play.golang.org/p/l9CeGUXNwZP
```go
package main

import (
	"fmt"
	"github.com/yaegashi/wtz.go"
	"time"
)

func main() {
	var n string
	var l *time.Location

	n = "Tokyo Standard Time"
	l, _ = wtz.LoadLocation(n)
	fmt.Printf("%v -> %v\n", n, l)

	l, _ = time.LoadLocation("America/Los_Angeles")
	n, _ = wtz.LocationToName(l)
	fmt.Printf("%v -> %v\n", l, n)
}
```

## Acknowledgement

[The mapping table](wtz_maps.go) is based on 
[windowsZones.xml](https://raw.githubusercontent.com/unicode-org/cldr/master/common/supplemental/windowsZones.xml)
from [Unicode CLDR](http://cldr.unicode.org/).
[The table generator](gen/genmaps.go) is inspired by [genzabbrs.go](https://golang.org/src/time/genzabbrs.go).


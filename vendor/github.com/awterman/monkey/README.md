# Monkey - Gomonkey Generic
[![Go Reference](https://pkg.go.dev/badge/github.com/awterman/monkey.svg)](https://pkg.go.dev/github.com/awterman/monkey)
[![Go Report Card](https://goreportcard.com/badge/github.com/awterman/monkey)](https://goreportcard.com/report/github.com/awterman/monkey)
[![Go Coverage](https://github.com/awterman/monkey/wiki/coverage.svg)](https://raw.githack.com/wiki/awterman/monkey/coverage.html)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

- Monkey is a generic implementation of the [gomonkey](https://github.com/agiledragon/gomonkey).
- Monkey can be used in testing for stubbing.
- Very easy to use: only 3 function in all.

## Usage:

Patch a function:

```go
	p := Func(nil, fnFoo, func() string { return "bar" })
	defer p.Reset()

	if fnFoo() != "bar" {
		t.Error("patch failed")
	}
```

You can also patch global variables and methods with `Var` and `Method`. Check the tests for example.

## Notes

- TL;DR: Be sure to add flag `-gcflags=-l`(below go1.10) or `-gcflags=all=-l`(go1.10 and above) in testing like `go test -gcflags=all=-l -v`
  - gomonkey fails to patch a function or a member method if inlining is enabled, please running your tests with inlining disabled by adding the command line argument that is -gcflags=-l(below go1.10) or -gcflags=all=-l(go1.10 and above).
- A panic may happen when a goroutine is patching a function or a member method that is visited by another goroutine at the same time. That is to say, gomonkey is **not threadsafe**. Do not call `t.Parallel` in your tests.

## Improvements Compared to the Original

- Generic implementation instead of interface{} to provide type checks especially for functions/methods, which also benefits the code completion a lot.
- Only 3 functions to achieve all benefits from gomonkey.

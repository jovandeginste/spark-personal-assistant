// Package ical parses iCalendar (.ics) files.
package ical

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/bounoable/ical/lex"
	"github.com/bounoable/ical/parse"
)

// Calendar is a parsed iCalendar.
type Calendar parse.Calendar

// Parse parses the iCalendar from r.
func Parse(r io.Reader, opts ...Option) (Calendar, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	cal, err := parse.Items(
		lex.Reader(r, cfg.lexerOptions...),
		cfg.parserOptions...,
	)

	if err != nil {
		return Calendar{}, err
	}

	return Calendar(cal), nil
}

// ParseFile parses the iCalendar from the file at filepath.
func ParseFile(filepath string, opts ...Option) (Calendar, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return Calendar{}, err
	}
	defer f.Close()
	return Parse(f, opts...)
}

// ParseText parses the iCalendar from text.
func ParseText(text string, opts ...Option) (Calendar, error) {
	return Parse(strings.NewReader(text), opts...)
}

// Option is a lex/parse option.
type Option func(*config)

// LexWith adds options to the lexer.
func LexWith(opts ...lex.Option) Option {
	return func(cfg *config) {
		cfg.lexerOptions = append(cfg.lexerOptions, opts...)
	}
}

// ParseWith adds options to the parser.
func ParseWith(opts ...parse.Option) Option {
	return func(cfg *config) {
		cfg.parserOptions = append(cfg.parserOptions, opts...)
	}
}

// Context adds a context to the lexer & parser.
func Context(ctx context.Context) Option {
	return func(cfg *config) {
		LexWith(lex.Context(ctx))(cfg)
		ParseWith(parse.Context(ctx))(cfg)
	}
}

type config struct {
	lexerOptions  []lex.Option
	parserOptions []parse.Option
}

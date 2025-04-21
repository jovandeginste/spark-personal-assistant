// Package lex tokenizes iCalendar files for the parser.
package lex

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const eof = rune(-1)

// Reader lexes the iCalendar from r and sends the tokens to the returned channel.
// Lex errors are sent to the Item channel as an Error item.
func Reader(r io.Reader, opts ...Option) <-chan Item {
	l := lexer{
		input: bufio.NewReader(r),
		items: make(chan Item),
	}

	for _, opt := range opts {
		opt(&l)
	}

	if l.ctx == nil {
		l.ctx = context.Background()
	}

	go func() {
		defer close(l.items)
		for state := lexContentLine; state != nil; {
			select {
			case <-l.ctx.Done():
				l.items <- Item{
					Type:  Error,
					Value: l.ctx.Err().Error(),
				}
				return
			default:
				state = state(&l)
			}
		}
	}()

	return l.items
}

// File lexes the iCalendar from the file at filepath.
func File(filepath string, opts ...Option) (<-chan Item, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Reader(f, opts...), nil
}

// Text lexes the iCalendar from the given text.
func Text(text string, opts ...Option) <-chan Item {
	return Reader(strings.NewReader(text))
}

// Option is a lexer option.
type Option func(*lexer)

// Context adds a context to the lexer.
func Context(ctx context.Context) Option {
	return func(l *lexer) {
		l.ctx = ctx
	}
}

// StrictLineBreaks enforces "CRLF" line breaks in the iCalendar source file.
// By default the lexer also allows "LF" line breaks.
func StrictLineBreaks(l *lexer) {
	l.strictLineBreaks = true
}

type lexer struct {
	ctx              context.Context
	strictLineBreaks bool
	input            io.RuneReader
	bufferedInput    string
	bufPos           int
	width            int
	consumed         int
	items            chan Item
}

type stateFunc func(*lexer) stateFunc

func (l *lexer) emit(t ItemType) {
	l.items <- Item{
		Type:  t,
		Value: l.bufferedInput[:l.bufPos],
	}
	l.ignore()
}

func (l *lexer) emitIf(cond bool, t ItemType) {
	if cond {
		l.emit(t)
	}
}

func (l *lexer) emitAdvanced(t ItemType) {
	l.emitIf(l.bufPos > 0, t)
}

func (l *lexer) emitEOF() {
	l.ignore()
	l.emit(EOF)
}

func (l *lexer) advance(n int) {
	l.bufPos += n
}

func (l *lexer) next() (r rune) {
	for l.bufPos >= len(l.bufferedInput) {
		err := l.readRune()
		if err == nil {
			continue
		}

		if errors.Is(err, io.EOF) {
			break
		}

		l.items <- Item{
			Type:  Error,
			Value: err.Error(),
		}
		break
	}

	if l.bufPos >= len(l.bufferedInput) {
		l.width = 0
		return eof
	}

	r, l.width = utf8.DecodeRuneInString(l.bufferedInput[l.bufPos:])
	l.bufPos += l.width

	return
}

func (l *lexer) readRune() error {
	r, _, err := l.input.ReadRune()
	if err != nil {
		return err
	}

	// if first rune is not one of [CR, LF], add it to the input and return
	if r != cr && r != lf {
		l.bufferedInput += string(r)
		return nil
	}

	r2, _, err := l.input.ReadRune()
	if err != nil {
		return err
	}

	// if the first rune is LF and the second is a space, unfold by skipping these two runes
	if r == lf && unicode.IsSpace(r2) {
		return nil
	}

	// if r + r2 != CRLF, add both runes to the input
	if !(r == cr && r2 == lf) {
		l.bufferedInput += string(r) + string(r2)
		return nil
	}

	r3, _, err := l.input.ReadRune()
	if err != nil {
		return err
	}

	// r = CR, r2 = LF
	// if r3 is not a space, add a CRLF line break and r3 to the input
	if !unicode.IsSpace(r3) {
		l.bufferedInput += string(r) + string(r2) + string(r3)
		return nil
	}

	// r + r2 = CRLF, r3 = SPACE -> drop all three runes
	return nil
}

func (l *lexer) ignore() {
	l.bufferedInput = l.bufferedInput[l.bufPos:]
	l.consumed += l.bufPos
	l.bufPos = 0
}

func (l *lexer) backup() {
	l.bufPos -= l.width
}

func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) hasPrefix(prefix string) bool {
	for len(prefix) > len(l.bufferedInput[l.bufPos:]) {
		err := l.readRune()
		if err == nil {
			continue
		}

		if errors.Is(err, io.EOF) {
			break
		}

		l.items <- Item{
			Type:  Error,
			Value: err.Error(),
		}
		return false
	}

	return strings.HasPrefix(l.bufferedInput[l.bufPos:], prefix)
}

func (l *lexer) errorf(format string, args ...interface{}) stateFunc {
	l.items <- Item{
		Type:  Error,
		Value: fmt.Sprintf(format, args...),
	}
	return nil
}

func (l *lexer) unexpected(r rune, valid ...rune) stateFunc {
	svalid := make([]string, len(valid))
	for i, r := range valid {
		svalid[i] = string(r)
	}
	return l.errorf("expected character at pos %d to be one of %s; got %s", l.pos(), svalid, string(r))
}

func (l *lexer) unexpectedEOF() stateFunc {
	return l.errorf("unexpected EOF at pos %d", l.pos())
}

func (l *lexer) pos() int {
	return l.consumed + l.bufPos
}

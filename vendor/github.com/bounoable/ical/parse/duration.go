package parse

import (
	"errors"
	"fmt"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"
)

func parseDuration(raw string) (time.Duration, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	return (&durationParser{value: raw}).parse()
}

type durationParser struct {
	value string
	pos   int
	width int
}

const day = time.Hour * 24
const week = day * 7

// dur-value  = (["+"] / "-") "P" (dur-date / dur-time / dur-week)
// dur-date   = dur-day [dur-time]
// dur-time   = "T" (dur-hour / dur-minute / dur-second)
// dur-week   = 1*DIGIT "W"
// dur-hour   = 1*DIGIT "H" [dur-minute]
// dur-minute = 1*DIGIT "M" [dur-second]
// dur-second = 1*DIGIT "S"
// dur-day    = 1*DIGIT "D"
func (p *durationParser) parse() (time.Duration, error) {
	r, err := p.next()
	if err != nil {
		return 0, p.unexpectedEnd()
	}

	multiplier := time.Duration(1)

	if r == '-' {
		multiplier = -1
	} else if r != '+' {
		p.backup()
	}

	if r, err = p.next(); r != 'P' {
		return 0, fmt.Errorf("expected 'P' at pos %d; got %s", p.pos, string(r))
	}

	if r, err = p.next(); err != nil {
		return 0, p.unexpectedEnd()
	}

	if r == 'T' {
		dur, err := p.parseTime()
		if err != nil {
			return 0, fmt.Errorf("failed to parse time duration: %w", err)
		}
		return dur * multiplier, nil
	}
	p.backup()

	num, err := p.parseDigits()
	if err != nil {
		return 0, fmt.Errorf("failed to parse digits: %w", err)
	}

	if r, err = p.next(); err != nil {
		return 0, p.unexpectedEnd()
	}

	switch r {
	case 'W':
		return week * time.Duration(num) * multiplier, nil
	case 'D':
		dayDur := day * time.Duration(num)

		if r, err = p.next(); err != nil {
			return dayDur * multiplier, nil
		}

		if r == 'T' {
			timeDur, err := p.parseTime()
			if err != nil {
				return 0, err
			}
			return (dayDur + timeDur) * multiplier, nil
		}

		return 0, fmt.Errorf("unexpected %s at pos %d", string(r), p.pos)
	default:
		return 0, fmt.Errorf("expected one of [W D] at pos %d; got %s", p.pos, string(r))
	}
}

func (p *durationParser) parseTime() (time.Duration, error) {
	var r rune
	var total time.Duration

	for {
		num, err := p.parseDigits()
		if err != nil {
			return 0, fmt.Errorf("failed to parse digits: %w", err)
		}

		if r, err = p.next(); err != nil {
			return 0, p.unexpectedEnd()
		}

		var one time.Duration

		switch r {
		case 'H':
			one = time.Hour
		case 'M':
			one = time.Minute
		case 'S':
			one = time.Second
		default:
			return 0, fmt.Errorf("expected one of [H M S] at pos %d; got %s", p.pos, string(r))
		}

		total += one * time.Duration(num)

		if r, err = p.next(); err != nil {
			return total, nil
		}
		p.backup()
	}
}

func (p *durationParser) parseDigits() (int, error) {
	var r rune
	var err error
	var digits string

	if r, err = p.next(); err != nil {
		return 0, p.unexpectedEnd()
	}

	for unicode.IsDigit(r) {
		digits += string(r)
		if r, err = p.next(); err != nil {
			return 0, p.unexpectedEnd()
		}
	}
	p.backup()

	num, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("string to int conversion failed: %w", err)
	}

	return num, nil
}

var errEndOfDuration = errors.New("end of duration")

func (p *durationParser) next() (rune, error) {
	if p.pos >= len(p.value) {
		p.width = 0
		return 0, errEndOfDuration
	}
	r, w := utf8.DecodeRuneInString(p.value[p.pos:])
	p.width = w
	p.pos += w
	return r, nil
}

func (p *durationParser) backup() {
	p.pos -= p.width
}

func (p *durationParser) unexpectedEnd() error {
	return fmt.Errorf("unexpected end of duration at pos %d", p.pos)
}

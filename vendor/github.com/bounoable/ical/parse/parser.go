package parse

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bounoable/ical/lex"
)

var errEndOfItems = errors.New("end of items")

// Error is a parser error.
type Error struct {
	Err error
}

func (err *Error) Error() string {
	return fmt.Sprintf("parse: %v", err.Err)
}

func (err *Error) Unwrap() error {
	return err.Err
}

// Items parses a channel of lex.Item, returns the parsed iCalendar and/or an *Error if it fails.
func Items(items <-chan lex.Item, opts ...Option) (Calendar, error) {
	p := parser{items: items}
	for _, opt := range opts {
		opt(&p)
	}
	if p.ctx == nil {
		p.ctx = context.Background()
	}
	return p.parse()
}

// Slice parses a slice of lex.Item.
func Slice(items []lex.Item, opts ...Option) (Calendar, error) {
	ch := make(chan lex.Item)
	go func() {
		defer close(ch)
		for _, item := range items {
			ch <- item
		}
	}()
	return Items(ch, opts...)
}

// Option is a parser option.
type Option func(*parser)

// Context adds a context to the parser.
func Context(ctx context.Context) Option {
	return func(p *parser) {
		p.ctx = ctx
	}
}

// Location configures loc to be used as the *time.Location for parsing
// date / datetime values that don't explicitly have "UTC" set as the timezone
// by the "Z" suffix.
func Location(loc *time.Location) Option {
	return func(p *parser) {
		p.loc = loc
	}
}

// InclusiveEnds configures the parser to add 1 day to the "End" time field
// of every event with a DTEND property value of type DATE.
func InclusiveEnds(p *parser) {
	p.inclusiveEnds = true
}

type parser struct {
	ctx           context.Context
	loc           *time.Location
	inclusiveEnds bool

	items     <-chan lex.Item
	buf       [2]lex.Item
	start     int
	pos       int
	peekCount int

	cal Calendar
}

func (p *parser) nextItem() (lex.Item, error) {
	item, ok := <-p.items
	if !ok {
		return item, errEndOfItems
	}
	return item, nil
}

func (p *parser) next() (lex.Item, error) {
	select {
	case <-p.ctx.Done():
		return lex.Item{}, p.ctx.Err()
	default:
	}

	if p.peekCount > 0 {
		p.peekCount--
	} else {
		var err error
		if p.buf[0], err = p.nextItem(); err != nil {
			return lex.Item{}, err
		}
	}
	return p.buf[p.peekCount], nil
}

func (p *parser) nextType(typ lex.ItemType) (lex.Item, error) {
	item, err := p.next()
	if err != nil {
		return item, err
	}

	if item.Type != typ {
		return item, p.unexpectedType(item, typ)
	}

	return item, nil
}

func (p *parser) peek() (lex.Item, error) {
	if p.peekCount > 0 {
		return p.buf[p.peekCount-1], nil
	}
	p.peekCount = 1
	var err error
	if p.buf[0], err = p.nextItem(); err != nil {
		return lex.Item{}, err
	}
	return p.buf[0], nil
}

func (p *parser) backup() {
	p.peekCount++
}

func (p *parser) errorf(format string, vals ...interface{}) error {
	return fmt.Errorf(format, vals...)
}

func (p *parser) unexpectedType(item lex.Item, expected lex.ItemType) error {
	return p.errorf("expected item of type %v; got %s", expected, item)
}

func (p *parser) parse() (Calendar, error) {
	if err := p.parseCalendar(); err != nil {
		return p.cal, &Error{Err: err}
	}
	return p.cal, nil
}

func (p *parser) parseCalendar() error {
	item, err := p.next()
	if err != nil {
		return err
	}

	if item.Type != lex.CalendarBegin {
		return p.unexpectedType(item, lex.CalendarBegin)
	}

	cal := Calendar{
		Calscale: "GREGORIAN",
	}

loop:
	for {
		item, err = p.next()
		if err != nil {
			return err
		}

		switch item.Type {
		case lex.CalendarEnd:
			break loop
		case lex.EventBegin:
			p.backup()
			evt, err := p.parseEvent()
			if err != nil {
				return err
			}
			cal.Events = append(cal.Events, evt)
		case lex.Name:
			p.backup()
			prop, err := p.parseProperty()
			if err != nil {
				return err
			}
			cal.Properties = append(cal.Properties, prop)
		default:
			return p.errorf("unexpected item of type %s", item.Type)
		}
	}

	if item.Type != lex.CalendarEnd {
		return p.unexpectedType(item, lex.CalendarEnd)
	}

	for _, prop := range cal.Properties {
		switch prop.Name {
		case "VERSION":
			cal.Version = prop.Value
		case "METHOD":
			cal.Method = prop.Value
		case "PRODID":
			cal.ProductID = prop.Value
		}
	}

	p.cal = cal

	return nil
}

func (p *parser) parseEvent() (Event, error) {
	var evt Event
	item, err := p.nextType(lex.EventBegin)
	if err != nil {
		return evt, err
	}

loop:
	for {
		item, err = p.next()
		if err != nil {
			return evt, err
		}

		switch item.Type {
		case lex.EventEnd:
			p.backup()
			break loop
		case lex.AlarmBegin:
			p.backup()
			alarm, err := p.parseAlarm()
			if err != nil {
				return evt, fmt.Errorf("failed to parse alarm: %w", err)
			}
			evt.Alarms = append(evt.Alarms, alarm)
			continue
		default:
		}

		if item.Type != lex.Name {
			return evt, p.unexpectedType(item, lex.Name)
		}

		p.backup()
		prop, err := p.parseProperty()
		if err != nil {
			return evt, err
		}
		evt.Properties = append(evt.Properties, prop)
	}

	if item, err = p.nextType(lex.EventEnd); err != nil {
		return evt, err
	}

	for _, prop := range evt.Properties {
		switch prop.Name {
		case "UID":
			evt.UID = prop.Value
		case "DTSTART":
			t, err := p.parseTime(prop)
			if err != nil {
				return evt, err
			}
			evt.Start = t
		case "DTEND":
			t, err := p.parseDTEND(prop)
			if err != nil {
				return evt, err
			}
			evt.End = t
		case "DTSTAMP":
			t, err := p.parseTime(prop)
			if err != nil {
				return evt, err
			}
			evt.Timestamp = t
		case "SUMMARY":
			evt.Summary = prop.Value
		case "DESCRIPTION":
			evt.Description = prop.Value
		}
	}

	if err := evt.finalize(); err != nil {
		return evt, err
	}

	return evt, nil
}

func (p *parser) parseAlarm() (Alarm, error) {
	var alarm Alarm

	item, err := p.nextType(lex.AlarmBegin)
	if err != nil {
		return alarm, err
	}

	for {
		item, err = p.next()
		if err != nil {
			return alarm, err
		}

		if item.Type == lex.AlarmEnd {
			p.backup()
			break
		}

		if item.Type != lex.Name {
			return alarm, p.unexpectedType(item, lex.Name)
		}

		p.backup()
		prop, err := p.parseProperty()
		if err != nil {
			return alarm, err
		}
		alarm.Properties = append(alarm.Properties, prop)
	}

	if item, err = p.nextType(lex.AlarmEnd); err != nil {
		return alarm, err
	}

	for _, prop := range alarm.Properties {
		switch prop.Name {
		case "TRIGGER":
			alarm.Trigger = prop.Value
		case "ACTION":
			alarm.Action = prop.Value
		}
	}

	return alarm, nil
}

func (p *parser) parseProperty() (Property, error) {
	var name string
	params := make(Parameters)

	item, err := p.nextType(lex.Name)
	if err != nil {
		return Property{}, err
	}
	name = item.Value

	if item, err = p.next(); err != nil {
		return Property{}, err
	}

	if item.Type == lex.ParamName {
		p.backup()
		if err = p.parseParams(params); err != nil {
			return Property{}, err
		}
		if item, err = p.nextType(lex.Value); err != nil {
			return Property{}, err
		}
	}

	if item.Type != lex.Value {
		return Property{}, p.unexpectedType(item, lex.Value)
	}

	return Property{
		Name:   name,
		Params: params,
		Value:  item.Value,
	}, nil
}

func (p *parser) parseParams(params Parameters) error {
	for {
		item, err := p.next()
		if err != nil {
			return err
		}

		if item.Type != lex.ParamName {
			p.backup()
			break
		}

		name := item.Value
		var values []string

		for {
			item, err = p.next()
			if err != nil {
				return err
			}

			if item.Type != lex.ParamValue {
				p.backup()
				break
			}

			values = append(values, item.Value)
		}

		params[name] = values
	}

	return nil
}

const (
	layoutDate          = "20060102"
	layoutDateTimeUTC   = "20060102T150405Z"
	layoutDateTimeLocal = "20060102T150405"
)

func (p *parser) parseDTEND(prop Property) (time.Time, error) {
	if len(prop.Value) != len(layoutDate) {
		return p.parseTime(prop)
	}

	t, err := p.parseTime(prop)
	if err != nil || !p.inclusiveEnds {
		return t, err
	}

	return t.AddDate(0, 0, 1), nil
}

func (p *parser) parseTime(prop Property) (time.Time, error) {
	prop.Value = normalizeDateTimeValue(prop.Value)

	var layout string
	loc := time.Local

	if strings.HasSuffix(prop.Value, "Z") {
		layout = layoutDateTimeUTC
		loc = time.UTC
	} else {
		layout = parseLayout(prop)

		if p.loc != nil {
			loc = p.loc
		} else if tzRaw, ok := prop.Params["TZID"]; ok {
			for _, raw := range tzRaw {
				if tzloc, err := time.LoadLocation(raw); err == nil {
					loc = tzloc
					break
				}
			}
		}
	}

	if layout == layoutDate && len(prop.Value) != len(layout) {
		layout = layoutDateTimeLocal
	}

	return time.ParseInLocation(layout, prop.Value, loc)
}

func parseLayout(prop Property) string {
	var layout string

	for name, values := range prop.Params {
		if name != "VALUE" {
			continue
		}

		for _, val := range values {
			switch val {
			case "DATE-TIME":
				layout = layoutDateTimeLocal
			default:
				layout = layoutDate
			}
		}

		break
	}

	if len(layout) != len(prop.Value) {
		switch len(prop.Value) {
		case len(layoutDate):
			layout = layoutDate
		case len(layoutDateTimeLocal):
			layout = layoutDateTimeLocal
		case len(layoutDateTimeUTC):
			layout = layoutDateTimeUTC
		}
	}

	return layout
}

var (
	shortTimeLayoutRE = regexp.MustCompile(`([0-9]+T)([0-9]{3,5})(Z?)$`)
)

func normalizeDateTimeValue(val string) string {
	return replaceAllStringSubmatchFunc(shortTimeLayoutRE, val, func(groups []string) string {
		if len(groups) < 4 {
			return val
		}

		timeVal := groups[2]
		switch len(timeVal) {
		case 3: // Hms
			timeVal = fmt.Sprintf("0%s0%s0%s", string(timeVal[0]), string(timeVal[1]), string(timeVal[2]))
		case 4: // HHms | Hmms
			timeVal = normalizeTimeValue(timeVal, 4)
		case 5: // HHmms
			timeVal = normalizeTimeValue(timeVal, 5)
		default:
			return val
		}

		return fmt.Sprintf("%s%s%s", groups[1], timeVal, groups[3])
	})
}

func replaceAllStringSubmatchFunc(re *regexp.Regexp, str string, repl func([]string) string) string {
	var result string
	var lastIndex int
	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []string{}
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}
		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}
	return result + str[lastIndex:]
}

var (
	now = time.Now()
)

func normalizeTimeValue(val string, digits int) string {
	var hour, minute, second, offset, foundCount int
	found := func() bool { return foundCount >= (digits - 3) }

	if hour, _ = strconv.Atoi(val[offset : offset+2]); hour < 24 {
		foundCount++
		offset += 2
	} else {
		hour, _ = strconv.Atoi(val[offset : offset+1])
		offset++
	}

	if !found() {
		if minute, _ = strconv.Atoi(val[offset : offset+2]); minute < 60 {
			foundCount++
			offset += 2
		} else {
			minute, _ = strconv.Atoi(val[offset : offset+1])
			offset++
		}
	} else {
		minute, _ = strconv.Atoi(val[offset : offset+1])
		offset++
	}

	if !found() {
		if second, _ = strconv.Atoi(val[offset : offset+2]); second < 60 {
			foundCount++
			offset += 2
		} else {
			second, _ = strconv.Atoi(val[offset : offset+1])
			offset++
		}
	} else {
		second, _ = strconv.Atoi(val[offset : offset+1])
		offset++
	}

	return time.
		Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, time.UTC).
		Format("150405")
}

package parse

import (
	"time"
)

// Calendar is a parsed iCalendar.
type Calendar struct {
	// Raw calendar properties
	Properties []Property
	// Product Identifier (https://tools.ietf.org/html/rfc5545#section-3.7.3)
	ProductID string
	// iCalendar Version (https://tools.ietf.org/html/rfc5545#section-3.7.4)
	Version string
	// Calendar Scale (https://tools.ietf.org/html/rfc5545#section-3.7.1)
	Calscale string
	// iCalendar object method (https://tools.ietf.org/html/rfc5545#section-3.7.2)
	Method string
	Events []Event
}

// Event is a parsed iCalendar event.
type Event struct {
	// Raw event properties
	Properties  []Property
	UID         string
	Alarms      []Alarm
	Timestamp   time.Time
	Start       time.Time
	End         time.Time
	Summary     string
	Description string
}

// Alarm is a parsed iCalendar alarm.
type Alarm struct {
	Properties []Property
	Action     string
	Trigger    string
}

// Property is an iCalendar property / content-line.
type Property struct {
	Name   string
	Params Parameters
	Value  string
}

// Parameters are the parameters of a Property.
type Parameters map[string][]string

// Contains determines if the values of the parameter with the given name contains val.
func (params Parameters) Contains(name string, val string) bool {
	for pname, vals := range params {
		if pname != name {
			continue
		}

		for _, pval := range vals {
			if pval == val {
				return true
			}
		}
	}
	return false
}

// Property returns the Property with the given name.
func (evt Event) Property(name string) (Property, bool) {
	for _, prop := range evt.Properties {
		if prop.Name == name {
			return prop, true
		}
	}
	return Property{}, false
}

func (evt *Event) finalize() error {
	if err := evt.applyDuration(); err != nil {
		return err
	}

	evt.applyImplicitOneDayDuration()
	evt.applyImplicitEndOfDayDuration()
	return nil
}

func (evt *Event) applyDuration() error {
	if _, ok := evt.Property("DTEND"); ok {
		return nil
	}

	prop, ok := evt.Property("DURATION")
	if !ok {
		return nil
	}

	dur, err := parseDuration(prop.Value)
	if err != nil {
		return err
	}
	evt.End = evt.Start.Add(dur)

	return nil
}

func (evt *Event) applyImplicitOneDayDuration() {
	// For cases where a "VEVENT" calendar component
	// specifies a "DTSTART" property with a DATE value type but no
	// "DTEND" nor "DURATION" property, the event's duration is taken to
	// be one day.

	if dtstart, ok := evt.Property("DTSTART"); !ok ||
		!(len(dtstart.Params["VALUE"]) == 0 ||
			dtstart.Params.Contains("VALUE", "DATE")) {
		return
	}

	if _, ok := evt.Property("DTEND"); ok {
		return
	}

	if _, ok := evt.Property("DURATION"); ok {
		return
	}

	evt.End = evt.Start.Add(time.Hour * 24)
}

func (evt *Event) applyImplicitEndOfDayDuration() {
	// For cases where a "VEVENT" calendar component
	// specifies a "DTSTART" property with a DATE-TIME value type but no
	// "DTEND" property, the event ends on the same calendar date and
	// time of day specified by the "DTSTART" property.

	if dtstart, ok := evt.Property("DTSTART"); !ok || !dtstart.Params.Contains("VALUE", "DATE-TIME") {
		return
	}

	if _, ok := evt.Property("DTEND"); ok {
		return
	}

	evt.End = time.Date(
		evt.Start.Year(),
		evt.Start.Month(),
		evt.Start.Day(),
		0, 0, 0, 0,
		evt.Start.Location(),
	).AddDate(0, 0, 1)
}

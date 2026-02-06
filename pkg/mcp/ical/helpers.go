package ical

import (
	"strings"
	"time"

	"github.com/apognu/gocal"
	"github.com/yaegashi/wtz.go"
)

func parseICalRawDate(rs *gocal.RawDate, start *time.Time) (time.Time, error) {
	if v, ok := rs.Params["VALUE"]; ok {
		if v == "DATE" {
			return parseICalDate(rs)
		}
	}

	return parseICalTime(rs, start)
}

func parseICalDate(rs *gocal.RawDate) (time.Time, error) {
	return time.Parse("20060102", rs.Value)
}

func parseICalTime(rs *gocal.RawDate, start *time.Time) (time.Time, error) {
	ts, ok := rs.Params["TZID"]
	if !ok {
		return *start, nil
	}

	l := parseTimezone(ts)

	return time.ParseInLocation("20060102T150405", start.Format("20060102T150405"), l)
}

func parseTimezone(tz string) *time.Location {
	if l, err := wtz.LoadLocation(tz); err == nil {
		return l
	}

	if l, err := time.LoadLocation(tz); err == nil {
		return l
	}

	return time.UTC
}

func cleanDuration(d time.Duration) string {
	d = d.Round(60 * time.Second)
	if d == 0 {
		return "no duration"
	}

	r := d.String()
	r = strings.TrimSuffix(r, "0s")
	r = strings.TrimSuffix(r, "0m")

	return r
}

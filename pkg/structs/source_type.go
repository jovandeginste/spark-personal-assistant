package structs

import (
	"errors"
	"strings"
)

type SourceType string

func (st *SourceType) String() string {
	return string(*st)
}

func (st *SourceType) Set(v string) error {
	v = strings.ToLower(v)
	switch v {
	case "local", "json", "ical", "vcf", "weather", "rss":
		*st = SourceType(v)
		return nil
	default:
		return errors.New(`must be one of: "local", "json", "ical", "vcf", "weather", "rss"`)
	}
}

func (st *SourceType) Type() string {
	return "SourceType"
}

func (s SourceType) RequiredFields() []string {
	switch s {
	case SourceTypeJSON, SourceTypeICal, SourceTypeRSS, SourceTypeVCF:
		return []string{"url"}
	case SourceTypeWeather:
		return []string{"location"}
	default:
		return []string{}
	}
}

func NewSourceType(s string) (SourceType, error) {
	var st SourceType

	return st, st.Set(s)
}

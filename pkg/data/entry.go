package data

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aquasecurity/table"
	"gorm.io/gorm"
)

type Importance string

var ErrInvalidImportance = errors.New("invalid importance")

const (
	LOW    Importance = "low"
	MEDIUM Importance = "medium"
	HIGH   Importance = "high"
)

type Entry struct {
	ID         uint64         `gorm:"primaryKey" json:"-"`
	RemoteID   string         `gorm:"not null;uniqueIndex:idx_source_id" json:"-"`
	Date       HumanTime      `gorm:"not null;index"`
	Importance Importance     `gorm:"not null" json:",omitempty"`
	SourceID   uint64         `gorm:"not null;uniqueIndex:idx_source_id" json:"-"`
	Summary    string         `gorm:"not null"`
	Metadata   map[string]any `gorm:"serializer:json" json:",omitempty"`

	DateString string `gorm:"-" json:"-"`

	Source *Source `json:",omitempty"`
}

func (e *Entry) SetMetadata(key string, value any) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}

	e.Metadata[key] = value
}

func (e *Entry) SetMetadataIfNotEmpty(key string, value any) {
	switch v := value.(type) {
	case string:
		uv, err := strconv.Unquote("\"" + v + "\"")
		if err == nil {
			uv = strings.TrimSpace(uv)
		} else {
			uv = strings.TrimSpace(v)
		}

		value = uv
	}

	switch value {
	case nil, "", 0:
		return
	}

	e.SetMetadata(key, value)
}

func (e *Entry) GenerateRemoteID() {
	if e.RemoteID != "" {
		return
	}

	e.RemoteID = e.NewRemoteID()
}

func (e *Entry) BeforeSave(_ *gorm.DB) error {
	e.GenerateRemoteID()

	return nil
}

func (e *Entry) AfterFind(_ *gorm.DB) error {
	e.DateString = e.FormattedDate()
	return nil
}

func (e *Entry) NewRemoteID() string {
	return generateHash(fmt.Sprintf(
		"%d\n%s",
		e.Date.UTC().Unix(), e.Summary,
	))
}

func generateHash(s string) string {
	hasher := sha512.New()
	fmt.Fprint(hasher, s)

	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (e *Entry) FormattedDate() string {
	return e.Date.FormatDate()
}

func parseDate(d string) (time.Time, error) {
	if d == "" {
		return time.Now().In(LocalTimezone).Truncate(24 * time.Hour), nil
	}

	return time.ParseInLocation("2006-01-02", d, LocalTimezone)
}

func (e *Entry) SetDate(d string) error {
	parsedDate, err := parseDate(d)
	if err != nil {
		return err
	}

	e.Date = HumanTime{parsedDate}

	return nil
}

func (e *Entry) SetImportance(i string) error {
	switch Importance(i) {
	case LOW:
		e.Importance = LOW
	case MEDIUM:
		e.Importance = MEDIUM
	case HIGH:
		e.Importance = HIGH
	default:
		return fmt.Errorf("%w: %s", ErrInvalidImportance, i)
	}

	return nil
}

func (e *Entry) PrintTo(w io.Writer) {
	t := table.New(w)

	t.AddRow("ID", strconv.FormatUint(e.ID, 10))
	t.AddRow("Remote ID", e.RemoteID)
	t.AddRow("Data", e.DateString)
	t.AddRow("Summary", e.Summary)
	t.AddRow("Importance", string(e.Importance))

	if e.Source != nil {
		t.AddRow("Source", e.Source.Name)
	}

	for k, v := range e.Metadata {
		t.AddRow(k, fmt.Sprintf("%v", v))
	}

	t.Render()
}

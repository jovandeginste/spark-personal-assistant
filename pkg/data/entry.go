package data

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/aquasecurity/table"
	"gorm.io/gorm"
)

type Importance string

var ErrInvalidImportance = fmt.Errorf("invalid importance")

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
	hasher := sha512.New()
	fmt.Fprintf(
		hasher,
		"%d\n%s",
		e.Date.Unix(), e.Summary,
	)

	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (e *Entry) FormattedDate() string {
	return e.Date.FormatDate()
}

func parseDate(d string) (time.Time, error) {
	if d == "" {
		return time.Now().UTC().Round(24 * time.Hour), nil
	}

	return time.Parse("2006-01-02", d)
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

	t.AddRow("ID", fmt.Sprintf("%d", e.ID))
	t.AddRow("Remote ID", e.RemoteID)
	t.AddRow("Data", e.DateString)
	t.AddRow("Summary", e.Summary)
	t.AddRow("Importance", string(e.Importance))
	t.AddRow("Source", e.Source.Name)

	for k, v := range e.Metadata {
		t.AddRow(k, fmt.Sprintf("%v", v))
	}

	t.Render()
}

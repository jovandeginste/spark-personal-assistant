package structs

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/aquasecurity/table"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/humantime"
	"gorm.io/gorm"
)

type Entry struct {
	ID       uint64              `gorm:"primaryKey" json:"-"`
	RemoteID string              `gorm:"not null;uniqueIndex:idx_source_id" json:"-"`
	Date     humantime.HumanTime `gorm:"not null;index"`
	SourceID uint64              `gorm:"not null;uniqueIndex:idx_source_id" json:"-"`
	Summary  string              `gorm:"not null"`
	Metadata map[string]any      `gorm:"serializer:json" json:",omitempty"`

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
	value = generic.CleanValue(value)

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
	e.DateString = e.FormattedDate()

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
		return time.Now().In(humantime.LocalTimezone).Truncate(24 * time.Hour), nil
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", d, humantime.LocalTimezone); err == nil {
		return t, nil
	}

	return time.ParseInLocation("2006-01-02", d, humantime.LocalTimezone)
}

func (e *Entry) SetDate(d string) error {
	parsedDate, err := parseDate(d)
	if err != nil {
		return err
	}

	e.Date = humantime.HumanTime{Time: parsedDate}

	return nil
}

func (e *Entry) ToString(markdown bool) string {
	b := bytes.Buffer{}

	e.PrintTo(&b, markdown)

	return b.String()
}

func (e *Entry) PrintTo(w io.Writer, markdown bool) {
	t := table.New(w)
	defer t.Render()

	if markdown {
		t.SetDividers(table.MarkdownDividers)

		t.SetBorderTop(false)
		t.SetBorderBottom(false)
		t.SetRowLines(false)
	}

	t.AddRow("ID", strconv.FormatUint(e.ID, 10))
	t.AddRow("Remote ID", e.RemoteID)
	t.AddRow("Date", e.DateString)
	t.AddRow("Summary", e.Summary)

	if e.Source != nil {
		t.AddRow("Source", e.Source.Name)
	}

	for k, v := range e.Metadata {
		t.AddRow(k, fmt.Sprintf("%v", v))
	}
}

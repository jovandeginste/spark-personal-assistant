package structs

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/aquasecurity/table"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"gorm.io/gorm"
)

const (
	SourceTypeLocal   SourceType = "local"
	SourceTypeJSON    SourceType = "json"
	SourceTypeICal    SourceType = "ical"
	SourceTypeVCF     SourceType = "vcf"
	SourceTypeWeather SourceType = "weather"
	SourceTypeRSS     SourceType = "rss"
)

type (
	Sources []Source
	Source  struct {
		ID          uint64 `gorm:"primaryKey" json:"-"`
		Name        string `gorm:"not null;unique;type:varchar(32)"`
		Description string
		Type        SourceType
		Metadata    map[string]any `gorm:"serializer:json" json:",omitempty"`

		Entries Entries `json:"-"`

		logger *slog.Logger
	}
)

func (src *Source) SetLogger(l *slog.Logger) {
	src.logger = l
}

func (src *Source) HasRequiredFields() error {
	missing := []string{}

	for _, f := range src.Type.RequiredFields() {
		if _, ok := src.Metadata[f]; !ok {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
}

func (srcs Sources) PrintTo(w io.Writer) {
	t := table.New(w)
	defer t.Render()

	t.AddHeaders("ID", "Name", "Type", "Description", "Valid")

	for _, s := range srcs {
		v := "✅"
		if err := s.HasRequiredFields(); err != nil {
			v = "❌ " + err.Error()
		}

		t.AddRow(
			strconv.FormatUint(s.ID, 10),
			s.Name,
			s.Type.String(),
			s.Description,
			v,
		)
	}
}

func (src Source) PrintTo(w io.Writer) {
	t := table.New(w)
	defer t.Render()

	t.AddRow("ID", strconv.FormatUint(src.ID, 10))
	t.AddRow("Name", src.Name)
	t.AddRow("Type", src.Type.String())
	t.AddRow("Description", src.Description)

	for k, v := range src.Metadata {
		t.AddRow(k, fmt.Sprintf("%v", v))
	}
}

func (s *Source) UnsetMetadata(key string) {
	if s.Metadata == nil {
		return
	}

	delete(s.Metadata, key)
}

func (s *Source) SetMetadata(key string, value any) {
	if s.Metadata == nil {
		s.Metadata = make(map[string]any)
	}

	s.Metadata[key] = value
}

func (s *Source) SetMetadataIfNotEmpty(key string, value any) {
	value = generic.CleanValue(value)

	switch value {
	case nil, "", 0:
		s.UnsetMetadata(key)
		return
	}

	s.SetMetadata(key, value)
}

func (s *Source) AfterFind(_ *gorm.DB) (err error) {
	s.logger = slog.Default()
	return
}

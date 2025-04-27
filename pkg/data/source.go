package data

import (
	"fmt"
	"io"

	"github.com/aquasecurity/table"
)

type (
	Sources []Source
	Source  struct {
		ID          uint64 `gorm:"primaryKey" json:"-"`
		Name        string `gorm:"not null;unique;type:varchar(16)"`
		Description string

		Entries Entries `json:"-"`
	}
)

func (srcs Sources) PrintTo(w io.Writer) {
	t := table.New(w)
	defer t.Render()

	t.AddHeaders("ID", "Name", "Description")

	for _, s := range srcs {
		t.AddRow(
			fmt.Sprintf("%d", s.ID),
			s.Name,
			s.Description,
		)
	}
}

func (src Source) PrintTo(w io.Writer) {
	t := table.New(w)
	defer t.Render()

	t.AddRow("Name", src.Name)
	t.AddRow("Description", src.Description)
}

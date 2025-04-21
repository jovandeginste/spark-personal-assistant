package data

import (
	"fmt"
	"io"

	"github.com/aquasecurity/table"
)

type Entries []Entry

func (es Entries) PrintTo(w io.Writer) {
	t := table.New(w)
	t.AddHeaders("ID", "Date", "Title", "Importance", "Source")

	for _, entry := range es {
		t.AddRow(
			fmt.Sprintf("%d", entry.ID),
			entry.FormattedDate(),
			entry.Summary,
			string(entry.Importance),
			entry.Source.Name,
		)
	}

	t.Render()
}

package data

import (
	"io"
	"strconv"

	"github.com/aquasecurity/table"
)

type Entries []Entry

func (es Entries) PrintTo(w io.Writer) {
	t := table.New(w)
	t.AddHeaders("ID", "Date", "Title", "Importance", "Source")

	for _, entry := range es {
		t.AddRow(
			strconv.FormatUint(entry.ID, 10),
			entry.DateString,
			entry.Summary,
			string(entry.Importance),
			entry.Source.Name,
		)
	}

	t.Render()
}

package structs

import (
	"io"
	"strconv"

	"github.com/aquasecurity/table"
)

type Entries []Entry

func (es Entries) PrintTo(w io.Writer) {
	t := table.New(w)
	defer t.Render()

	t.AddHeaders("ID", "Date", "Title", "Source")

	for _, entry := range es {
		t.AddRow(
			strconv.FormatUint(entry.ID, 10),
			entry.DateString,
			entry.Summary,
			entry.Source.Name,
		)
	}
}

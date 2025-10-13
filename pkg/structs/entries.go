package structs

import (
	"io"
	"strconv"

	"github.com/aquasecurity/table"
)

type Entries []Entry

func (es Entries) PrintTo(w io.Writer, markdown bool) {
	t := table.New(w)
	defer t.Render()

	if markdown {
		t.SetDividers(table.MarkdownDividers)

		t.SetBorderTop(false)
		t.SetBorderBottom(false)
		t.SetRowLines(false)
	}

	t.AddHeaders("ID", "Date", "Title", "TODO", "Source")

	for _, entry := range es {
		t.AddRow(
			strconv.FormatUint(entry.ID, 10),
			entry.DateRange(),
			entry.Summary,
			entry.TodoString,
			entry.Source.Name,
		)
	}
}

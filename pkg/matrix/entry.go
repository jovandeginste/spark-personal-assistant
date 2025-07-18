package matrix

import (
	"github.com/jovandeginste/spark-personal-assistant/pkg/humantime"
	"github.com/jovandeginste/spark-personal-assistant/pkg/structs"
	"maunium.net/go/mautrix/id"
)

type entry struct {
	ID          uint64 `json:"id"`
	Action      string `json:"action"`
	Date        string `json:"date"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Name        string `json:"name"`
	IsTodo      bool   `json:"todo"`
	IsDone      bool   `json:"done"`
}

func (e *entry) ToEntry() *structs.Entry {
	de := &structs.Entry{
		ID:      e.ID,
		Summary: e.Summary,
		IsTodo:  e.IsTodo,
		IsDone:  e.IsDone,
	}
	de.SetMetadata("person", e.Name)
	de.SetMetadata("description", e.Description)

	_ = de.SetDate(e.Date)

	return de
}

func (e *entry) Execute(mc *MatrixConfig, roomID id.RoomID, src *structs.Source) {
	de := e.ToEntry()
	if de == nil {
		return
	}

	de.Source = src

	switch e.Action {
	case "add":
		if err := mc.App.CreateEntry(de); err != nil {
			mc.App.Logger().Error("Failed to create entry", "error", err)
			return
		}

		deStr := de.ToString(true)
		mc.sendNotice(roomID, "Creating task:\n\n"+deStr)
		mc.AIData.AddChatHistory("assistant", "Created task:\n\n"+deStr)
	case "delete":
		eid, err := mc.App.FindEntryByRemoteID(mc.SourceID, de)
		if err != nil {
			mc.App.Logger().Error("Failed to find entry", "error", err)
			return
		}

		de.ID = eid

		if err := mc.App.DeleteEntry(de); err != nil {
			mc.App.Logger().Error("Failed to delete entry", "error", err)
			return
		}

		deStr := de.ToString(true)
		mc.sendNotice(roomID, "Deleting task:\n\n"+deStr)
		mc.AIData.AddChatHistory("assistant", "Deleted task:\n\n"+deStr)
	case "update":
		if de.ID == 0 {
			eid, err := mc.App.FindEntryByRemoteID(mc.SourceID, de)
			if err != nil {
				mc.App.Logger().Error("Failed to find entry", "error", err)
				return
			}

			de.ID = eid
		}

		if de.IsDone && de.DateEnd.IsZero() {
			de.DateEnd = *humantime.Now()
		}

		if err := mc.App.UpdateEntry(de); err != nil {
			mc.App.Logger().Error("Failed to update entry", "error", err)
			return
		}

		deStr := de.ToString(true)
		mc.sendNotice(roomID, "Updating task:\n\n"+deStr)
		mc.AIData.AddChatHistory("assistant", "Updating task:\n\n"+deStr)
	}
}

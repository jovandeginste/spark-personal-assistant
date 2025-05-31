package matrix

import (
	"fmt"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"maunium.net/go/mautrix/id"
)

type entry struct {
	Action      string `json:"action"`
	Date        string `json:"date"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Name        string `json:"name"`
}

func (e *entry) ToEntry() *data.Entry {
	de := &data.Entry{Summary: e.Summary}
	if err := de.SetDate(e.Date); err != nil {
		return nil
	}

	de.SetMetadata("description", e.Description)
	de.SetMetadata("person", e.Name)

	return de
}

func (e *entry) Execute(mc *MatrixConfig, roomID id.RoomID, src *data.Source) {
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
		mc.AIData.AddChatHistory("assistant", fmt.Sprintf("added task: %v", e))
		mc.sendMessage(roomID, fmt.Sprintf("Creating task: %+v", de))
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

		mc.AIData.AddChatHistory("assistant", fmt.Sprintf("deleted task: %v", e))
		mc.sendMessage(roomID, fmt.Sprintf("Deleting task: %+v", de))
	}
}

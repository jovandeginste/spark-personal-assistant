package app

import (
	"errors"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/structs"
	"gorm.io/gorm"
)

type EntryFilter struct {
	Source    *structs.Source
	DaysBack  uint
	DaysAhead uint
}

type ChatHistory struct {
	time    time.Time
	Role    string
	Content string
}

type AIData struct {
	ExtraContext     []string
	ChatHistory      []ChatHistory `json:",omitempty"`
	EmployerQuestion []string      `json:",omitempty"`
	UserData         UserData
	EntryFilter      *EntryFilter
	Entries          structs.Entries
}

func (aiData *AIData) ResetHistory() {
	aiData.ChatHistory = []ChatHistory{}
}

// CleanHistory: keep last 10 elements in aiData.ChatHistory:
func (aiData *AIData) CleanHistory() {
	h := []ChatHistory{}
	f := time.Now().Add(-1 * time.Hour)

	for i, e := range aiData.ChatHistory {
		if i < len(aiData.ChatHistory)-100 {
			continue
		}

		if e.time.Before(f) {
			continue
		}

		h = append(h, e)
	}

	aiData.ChatHistory = h
}

func (aiData *AIData) AddChatHistory(role string, input string) {
	aiData.ChatHistory = append(
		aiData.ChatHistory,
		ChatHistory{time: time.Now(), Role: role, Content: input},
	)
}

func (aiData *AIData) UpdateEntries(a *App) error {
	a.Logger().Info("Updating entries")

	entries, err := a.CurrentEntries(aiData.EntryFilter)
	if err != nil {
		return err
	}

	aiData.Entries = entries
	return nil
}

func (a *App) BuildData(ef *EntryFilter) (*AIData, error) {
	aiData := &AIData{
		ExtraContext: a.Config.ExtraContext,
		UserData:     a.Config.UserData,
		EntryFilter:  ef,
	}

	aiData.UpdateEntries(a)

	return aiData, nil
}

func (ef *EntryFilter) From() time.Time {
	return time.Now().Add(time.Duration(-ef.DaysBack*24) * time.Hour).Truncate(24 * time.Hour)
}

func (ef *EntryFilter) To() time.Time {
	return time.Now().Add(time.Duration(ef.DaysAhead*24) * time.Hour).Truncate(24 * time.Hour)
}

func (ef *EntryFilter) Query(q *gorm.DB) *gorm.DB {
	q = q.Where(
		q.Where(
			q.Where("date IS NULL").Where("is_todo = ?", true).Where("is_done = ?", false),
		).Or(
			q.Where("date <= ?", ef.To()).Where("date_end >= ?", ef.From()),
		),
	)

	if ef.Source == nil {
		return q
	}

	return q.Where("source_id = ?", ef.Source.ID)
}

func (a *App) CurrentEntries(ef *EntryFilter) (structs.Entries, error) {
	q := ef.Query(a.Query())

	var entries structs.Entries

	if err := q.Order("date ASC").Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func (a *App) Entries() (structs.Entries, error) {
	var entries structs.Entries

	if err := a.Query().
		Preload("Source").
		Order("date ASC").
		Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func (a *App) UpdateEntry(e *structs.Entry) error {
	return a.Query().Save(&e).Error
}

func (a *App) DeleteEntry(e *structs.Entry) error {
	return a.Query().Delete(&e).Error
}

func (a *App) FindEntry(e *structs.Entry) error {
	return a.Query().First(&e, e.ID).Error
}

func (a *App) FindEntryByRemoteID(sourceID uint64, e *structs.Entry) (uint64, error) {
	rid := e.NewRemoteID()

	var entry structs.Entry

	if err := a.Query().Where("source_id = ?", sourceID).Where("remote_id = ?", rid).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}

		return 0, err
	}

	return entry.ID, nil
}

func (a *App) Sources() (structs.Sources, error) {
	var sources structs.Sources

	if err := a.Query().Preload("Entries").Find(&sources).Error; err != nil {
		return nil, err
	}

	for _, s := range sources {
		s.SetLogger(a.Logger())
	}

	return sources, nil
}

func (a *App) CreateEntry(entry *structs.Entry) error {
	a.Logger().Info("Creating new entry", "date", entry.Date, "entry", entry.Summary, "source", entry.Source.Name)
	return a.Query().Create(entry).Error
}

func (a *App) DeleteSource(s *structs.Source) error {
	return a.Query().Select("Entries").Delete(&s).Error
}

func (a *App) FindSourceByName(name string) (*structs.Source, error) {
	source := structs.Source{Name: name}

	if err := a.Query().Where(&source).First(&source).Error; err != nil {
		return nil, err
	}

	source.SetLogger(a.Logger())

	return &source, nil
}

func (a *App) CreateSource(src *structs.Source) error {
	a.Logger().Info("Creating new source", "source", src.Name)
	return a.Query().Create(src).Error
}

func (a *App) UpdateSource(src *structs.Source) error {
	return a.Query().Save(&src).Error
}

func (a *App) FetchExistingEntries(sourceID uint64, entries structs.Entries) {
	for i, e := range entries {
		id, err := a.FindEntryByRemoteID(sourceID, &e)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}

			a.Logger().Error(err.Error())
		}

		entries[i].ID = id
	}
}

func (a *App) ReplaceSourceEntries(src *structs.Source, entries structs.Entries) error {
	a.Logger().Info("Replace entries for source", "entries", len(entries), "source", src.Name)

	for i := range entries {
		entries[i].SourceID = src.ID
	}

	if err := a.Query().Model(&structs.Entry{}).Save(entries).Error; err != nil {
		return err
	}

	return a.Query().Model(&src).Association("Entries").Unscoped().Replace(entries)
}

package app

import (
	"errors"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"gorm.io/gorm"
)

type EntryFilter struct {
	Source    *data.Source
	DaysBack  uint
	DaysAhead uint
}

func (ef *EntryFilter) From() time.Time {
	return time.Now().Add(time.Duration(-ef.DaysBack*24) * time.Hour).Truncate(24 * time.Hour)
}

func (ef *EntryFilter) To() time.Time {
	return time.Now().Add(time.Duration(ef.DaysAhead*24) * time.Hour).Truncate(24 * time.Hour)
}

func (ef *EntryFilter) Query(q *gorm.DB) *gorm.DB {
	q = q.Where("date >= ?", ef.From()).Where("date <= ?", ef.To())

	if ef.Source != nil {
		q = q.Where("source_id = ?", ef.Source.ID)
	}

	return q
}

func (a *App) CurrentEntries(ef EntryFilter) (data.Entries, error) {
	q := ef.Query(a.DB())

	var entries data.Entries

	if err := q.Order("date ASC").Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func (a *App) Entries() (data.Entries, error) {
	var entries data.Entries

	if err := a.DB().
		Preload("Source").
		Order("date ASC").
		Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func (a *App) DeleteEntry(e *data.Entry) error {
	return a.DB().Delete(&e).Error
}

func (a *App) FindEntry(e *data.Entry) error {
	return a.DB().First(&e, e.ID).Error
}

func (a *App) FindEntryByRemoteID(sourceID uint64, e *data.Entry) (uint64, error) {
	rid := e.NewRemoteID()

	var entry data.Entry

	if err := a.DB().Where("source_id = ?", sourceID).Where("remote_id = ?", rid).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}

		return 0, err
	}

	return entry.ID, nil
}

func (a *App) Sources() (data.Sources, error) {
	var sources data.Sources

	if err := a.DB().Preload("Entries").Find(&sources).Error; err != nil {
		return nil, err
	}

	return sources, nil
}

func (a *App) CreateEntry(entry *data.Entry) error {
	a.Logger().Info("Creating new entry", "date", entry.Date, "entry", entry.Summary, "source", entry.Source.Name)
	return a.DB().Create(entry).Error
}

func (a *App) DeleteSource(s *data.Source) error {
	return a.DB().Select("Entries").Delete(&s).Error
}

func (a *App) FindSourceByName(name string) (*data.Source, error) {
	source := data.Source{Name: name}

	if err := a.DB().Where(&source).First(&source).Error; err != nil {
		return nil, err
	}

	return &source, nil
}

func (a *App) CreateSource(src *data.Source) error {
	a.Logger().Info("Creating new source", "source", src.Name)
	return a.DB().Create(src).Error
}

func (a *App) FetchExistingEntries(sourceID uint64, entries data.Entries) {
	for i, e := range entries {
		id, err := a.FindEntryByRemoteID(sourceID, &e)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}

			a.logger.Error(err.Error())
		}

		entries[i].ID = id
	}
}

func (a *App) ReplaceSourceEntries(src *data.Source, entries data.Entries) error {
	a.Logger().Info("Replace entries for source", "entries", len(entries), "source", src.Name)

	for i := range entries {
		entries[i].SourceID = src.ID
	}

	if err := a.DB().Model(&data.Entry{}).Save(entries).Error; err != nil {
		return err
	}

	return a.DB().Model(&src).Association("Entries").Unscoped().Replace(entries)
}

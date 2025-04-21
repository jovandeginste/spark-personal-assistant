package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"gorm.io/gorm"
)

func (a *App) CurrentEntries() (data.Entries, error) {
	var entries data.Entries

	from := time.Now().Add(-7 * 24 * time.Hour)
	to := time.Now().Add(7 * 24 * time.Hour)

	if err := a.DB().
		Where("date >= ?", from).
		Where("date <= ?", to).
		Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func (a *App) Entries() (data.Entries, error) {
	var entries data.Entries

	if err := a.DB().Preload("Source").Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func (a *App) FindEntry(e *data.Entry) error {
	return a.DB().Where(&e).First(&e).Error
}

func (a *App) Sources() (data.Sources, error) {
	var sources data.Sources

	if err := a.DB().Preload("Entries").Find(&sources).Error; err != nil {
		return nil, err
	}

	return sources, nil
}

func (a *App) CreateEntry(entry data.Entry) error {
	entry.GenerateRemoteID()

	a.Logger().Info("Creating new entry", "date", entry.Date, "entry", entry.Summary, "source", entry.Source.Name)
	return a.DB().Create(&entry).Error
}

func (a *App) FindSourceByName(name string) (*data.Source, error) {
	source := data.Source{Name: name}

	if err := a.DB().Where(&source).First(&source).Error; err != nil {
		return nil, err
	}

	return &source, nil
}

func (a *App) CreateSource(src data.Source) error {
	a.Logger().Info("Creating new source", "source", src.Name)
	return a.DB().Create(&src).Error
}

func (a *App) FetchExistingEntries(entries data.Entries) {
	for i, e := range entries {
		if err := a.FindEntry(&e); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}

			a.logger.Error(err.Error())
		}

		entries[i] = e
	}
}

func (a *App) ReplaceSourceEntries(src *data.Source, entries data.Entries) error {
	a.Logger().Info("Replace entries for source", "entries", len(entries), "source", src.Name)

	for i := range entries {
		entries[i].SourceID = src.ID
		entries[i].GenerateRemoteID()
	}

	fmt.Printf("%#v\n", entries)

	a.DB().Model(&src).Association("Entries").Unscoped().Replace(entries)

	return nil
}

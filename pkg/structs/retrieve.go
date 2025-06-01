package structs

import (
	"errors"
	"fmt"

	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/mmcdole/gofeed"
)

var fp = gofeed.NewParser()

func (src *Source) RetrieveEntries() (Entries, error) {
	if err := src.HasRequiredFields(); err != nil {
		return nil, err
	}

	switch src.Type {
	case SourceTypeLocal:
		// nothing to do
		return nil, nil
	case SourceTypeJSON:
		return src.retrieveFromJSON()
	case SourceTypeRSS:
		return src.retrieveFromRSS()
	case SourceTypeWeather:
		return src.retrieveFromWeather()
	case SourceTypeICal:
		return src.retrieveFromICal()
	case SourceTypeVCF:
		return src.retrieveFromVCF()
	default:
		return nil, fmt.Errorf("unsupported source type: %s", src.Type)
	}
}

func (src *Source) retrieveFromJSON() (Entries, error) {
	url, ok := src.Metadata["url"].(string)
	if !ok {
		return nil, errors.New("missing url")
	}

	src.logger.Info("retrieving json", "url", url)

	return BuildEntriesFromJSON(url)
}

func (src *Source) retrieveFromRSS() (Entries, error) {
	url, ok := src.Metadata["url"].(string)
	if !ok {
		return nil, errors.New("missing url")
	}

	src.logger.Info("retrieving rss", "url", url)

	return BuildEntriesFromFeed(url)
}

func (src *Source) retrieveFromWeather() (Entries, error) {
	geocoder.SetClient(src.logger, "Spark")

	location, ok := src.Metadata["location"].(string)
	if !ok {
		return nil, errors.New("missing location")
	}

	src.logger.Info("retrieving weather", "location", location)

	return GetWeatherData(location)
}

func (src *Source) retrieveFromICal() (Entries, error) {
	url, ok := src.Metadata["url"].(string)
	if !ok {
		return nil, errors.New("missing url")
	}

	collection, ok := src.Metadata["collection"].(string)
	if !ok {
		collection = "calendar"
	}

	daysBack, ok := src.Metadata["days_back"].(uint)
	if !ok {
		daysBack = 120
	}

	daysAhead, ok := src.Metadata["days_ahead"].(uint)
	if !ok {
		daysAhead = 120
	}

	src.logger.Info("retrieving ical",
		"url", url,
		"collection", collection,
		"days_back", daysBack,
		"days_ahead", daysAhead,
	)

	return BuildEntriesFromRemote(url, daysBack, daysAhead, collection)
}

func (src *Source) retrieveFromVCF() (Entries, error) {
	url, ok := src.Metadata["url"].(string)
	if !ok {
		return nil, errors.New("missing url")
	}

	src.logger.Info("retrieving vcf", "url", url)

	return BuildEntriesFromFile(url)
}

package structs

import (
	"encoding/json"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
)

func BuildEntriesFromJSON(url string) (Entries, error) {
	r, err := generic.GetBody(url)
	if err != nil {
		return nil, err
	}

	var entries Entries

	if err := json.Unmarshal(r, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

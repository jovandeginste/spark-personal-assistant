package generic

import (
	"fmt"
	"sync"
	"time"
)

// failureCooldown is how long a failed HTTP resource is skipped before it is
// tried again.
const failureCooldown = 5 * time.Minute

var failedResources = struct {
	sync.Mutex
	until map[string]time.Time
}{
	until: make(map[string]time.Time),
}

func checkCooldown(uri string) error {
	failedResources.Lock()
	defer failedResources.Unlock()

	until, ok := failedResources.until[uri]
	if !ok {
		return nil
	}

	now := time.Now()
	if !now.Before(until) {
		delete(failedResources.until, uri)

		return nil
	}

	return fmt.Errorf("resource %s failed recently, retry in %s", uri, until.Sub(now).Round(time.Second))
}

func markFailure(uri string) {
	failedResources.Lock()
	defer failedResources.Unlock()

	now := time.Now()
	for key, until := range failedResources.until {
		if !now.Before(until) {
			delete(failedResources.until, key)
		}
	}

	failedResources.until[uri] = now.Add(failureCooldown)
}

func clearFailure(uri string) {
	failedResources.Lock()
	defer failedResources.Unlock()

	delete(failedResources.until, uri)
}

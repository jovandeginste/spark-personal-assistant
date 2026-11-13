package humantime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

var LocalTimezone = time.Local

type HumanTime struct {
	time.Time
}

func (ct *HumanTime) DateOnly() bool {
	return ct.Hour() == 0 && ct.Minute() == 0
}

func (ct *HumanTime) MarshalJSON() ([]byte, error) {
	// Handle nil pointer if necessary, though often you might want a zero time instead
	if ct == nil || ct.IsZero() {
		return []byte("null"), nil
	}

	// Format the time using the custom layout
	formatted := ct.FormatDate()

	// Return the formatted time as a JSON string (needs quotes)
	// Using fmt.Sprintf is a common way to ensure it's quoted
	jsonString := fmt.Sprintf(`"%s"`, formatted)

	return []byte(jsonString), nil
}

func (ct *HumanTime) UnmarshalJSON(data []byte) error {
	// Handle JSON null value: set the receiver to its zero value
	if bytes.Equal(data, []byte("null")) {
		*ct = HumanTime{} // Set the whole struct to its zero value
		return nil
	}

	// The input should be a JSON string. Unmarshal into a temporary string var.
	// This handles removing the surrounding quotes.
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("HumanTime.UnmarshalJSON: failed to unmarshal JSON string: %w", err)
	}

	// If the unquoted string is empty after unmarshalling, it's likely an invalid input
	// based on the MarshalJSON implementation (which outputs "null" or a formatted string, not "").
	// Treat an empty string as an error or zero value depending on desired strictness.
	// Returning an error is generally safer for unexpected input.
	if s == "" {
		// Could potentially return an error here, or set to zero value like null
		// fmt.Errorf("HumanTime.UnmarshalJSON: unexpected empty string input")
		*ct = HumanTime{} // Or treat empty string like null
		return nil
	}

	t := time.Time{}
	// Parse the unquoted string using the layout matching MarshalJSON's output
	var parseErr error

	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		t, parseErr = time.ParseInLocation(layout, s, LocalTimezone)
		if parseErr == nil {
			break
		}
	}

	if parseErr != nil {
		return fmt.Errorf("HumanTime.UnmarshalJSON: cannot parse %q: %w", s, parseErr)
	}

	// Set the parsed time on the receiver's internal time field
	ct.Time = t

	return nil // Success
}

func (ct *HumanTime) FormatDate() string {
	if ct.IsZero() {
		return "(no date)"
	}

	if ct.DateOnly() {
		return ct.In(LocalTimezone).Format("2006-01-02")
	}

	return ct.In(LocalTimezone).Format("2006-01-02 15:04")
}

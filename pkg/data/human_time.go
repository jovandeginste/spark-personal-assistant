package data

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

var LocalTimezone = time.Local

type HumanTime struct {
	time.Time
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
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		var err error

		t, err = time.ParseInLocation(layout, s, LocalTimezone)
		if err == nil {
			break
		}
	}

	if t.IsZero() {
		panic(s)
	}

	// Set the parsed time on the receiver's internal time field
	ct.Time = t

	return nil // Success
}

func (ct HumanTime) Value() (driver.Value, error) {
	// Check if the time is zero. You might want to store zero times as NULL in the database.
	if ct.IsZero() {
		return time.Time{}, nil // Return nil for zero time -> NULL in database
	}

	// Return the embedded time.Time. GORM/database driver knows how to handle this.
	return ct.Time, nil
}

func (ct *HumanTime) Scan(value any) error {
	// Scan needs a pointer receiver to modify the struct instance
	if value == nil {
		// Handle NULL from database
		ct.Time = time.Time{} // Set to zero value
		return nil
	}

	// Check if the value received from the database is time.Time
	// GORM drivers typically return time.Time for DATE/TIME/TIMESTAMP columns
	if t, ok := value.(time.Time); ok {
		ct.Time = t
		return nil
	}

	// If it's not time.Time, return an error
	// (You could add more type checks here if your driver returns dates differently,
	// e.g., as strings, but handling time.Time is the most common case)
	return fmt.Errorf("cannot scan type %T into HumanTime", value)
}

func (ct *HumanTime) FormatDate() string {
	if ct.Hour() == 0 && ct.Minute() == 0 {
		return ct.In(LocalTimezone).Format("2006-01-02")
	}

	return ct.In(LocalTimezone).Format("2006-01-02 15:04")
}

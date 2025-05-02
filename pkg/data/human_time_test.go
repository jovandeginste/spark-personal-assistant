//nolint:gocognit,funlen
package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Helper function to create a HumanTime with local time
func newHumanTimeLocal(year int, month time.Month, day, hour, minute, sec int) HumanTime {
	t := time.Date(year, month, day, hour, minute, sec, 0, localTimezone)
	return HumanTime{Time: t}
}

func TestHumanTime_MarshalJSON(t *testing.T) {
	localTimezone = time.FixedZone("Europe/Brussels", 2)
	defer func() { localTimezone = time.Local }()

	tests := []struct {
		name        string
		ht          *HumanTime
		expected    string
		expectError bool
	}{
		{
			name:     "Non-zero time with time part",
			ht:       &HumanTime{Time: time.Date(2023, time.October, 27, 10, 30, 0, 0, localTimezone)},
			expected: `"2023-10-27 10:30"`,
		},
		{
			name:     "Non-zero time without time part (midnight)",
			ht:       &HumanTime{Time: time.Date(2024, time.January, 15, 0, 0, 0, 0, localTimezone)},
			expected: `"2024-01-15"`,
		},
		{
			name:     "Zero time",
			ht:       &HumanTime{Time: time.Time{}},
			expected: `null`,
		},
		{
			name:     "Nil HumanTime pointer", // Although the code handles nil explicitly, standard JSON unmarshalling often doesn't produce a nil pointer directly. Still good to test the nil check.
			ht:       nil,
			expected: `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.ht)

			if tt.expectError {
				if err == nil {
					t.Errorf("MarshalJSON() expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("MarshalJSON() unexpected error: %v", err)
					return
				}

				if string(data) != tt.expected {
					t.Errorf("MarshalJSON() got = %s, want %s", string(data), tt.expected)
				}
			}
		})
	}
}

func TestHumanTime_UnmarshalJSON(t *testing.T) {
	localTimezone = time.FixedZone("Europe/Brussels", 2)
	defer func() { localTimezone = time.Local }()

	tests := []struct {
		name        string
		jsonInput   []byte
		expectedHT  HumanTime
		expectError bool
		expectPanic bool // Testing the panic behavior in the original code
	}{
		{
			name:      "Valid JSON string with date and time",
			jsonInput: []byte(`"2023-10-27 10:30"`),
			expectedHT: HumanTime{
				Time: time.Date(2023, time.October, 27, 10, 30, 0, 0, localTimezone),
			},
		},
		{
			name:      "Valid JSON string with date only",
			jsonInput: []byte(`"2024-01-15"`),
			expectedHT: HumanTime{
				Time: time.Date(2024, time.January, 15, 0, 0, 0, 0, localTimezone),
			},
		},
		{
			name:      "Valid JSON string with RFC3339",
			jsonInput: []byte(`"2023-10-27T10:30:00Z"`), // Example UTC RFC3339
			expectedHT: HumanTime{
				Time: time.Date(2023, time.October, 27, 10, 30, 0, 0, time.UTC),
			},
		},
		{
			name:        "JSON null",
			jsonInput:   []byte(`null`),
			expectedHT:  HumanTime{}, // Zero value
			expectError: false,
		},
		{
			name:        "Empty JSON string after unmarshalling (treated as zero)",
			jsonInput:   []byte(`""`),
			expectedHT:  HumanTime{}, // Code treats "" like null
			expectError: false,
		},
		{
			name:        "Invalid JSON format (not a string or null)",
			jsonInput:   []byte(`12345`),
			expectError: true, // json.Unmarshal fails
		},
		{
			name:        "Invalid time string format (leads to panic in current code)",
			jsonInput:   []byte(`"invalid-date"`),
			expectPanic: true, // Code panics if all parsing layouts fail
		},
		{
			name:        "Invalid JSON string format",
			jsonInput:   []byte(`abc`), // Missing quotes
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("UnmarshalJSON() panicked unexpectedly with: %v", r)
					}

					return
				}

				if tt.expectPanic {
					t.Errorf("UnmarshalJSON() did not panic as expected")
				}
			}()

			var ht HumanTime
			err := json.Unmarshal(tt.jsonInput, &ht)

			if tt.expectError {
				if err == nil {
					t.Errorf("UnmarshalJSON() expected an error but got none")
				}

				return
			}

			if tt.expectPanic {
				return
			}

			if err != nil {
				t.Errorf("UnmarshalJSON() unexpected error: %v", err)
				return
			}

			// Compare the time, ignoring location if necessary (though ParseInLocation uses Local)
			// Using Equal handles different representations of the same time moment.
			if !ht.Time.Equal(tt.expectedHT.Time) {
				t.Errorf("UnmarshalJSON() got time = %v, want %v", ht.Time, tt.expectedHT.Time)
			}

			// Also check the struct equality if the zero value is important
			if !reflect.DeepEqual(ht, tt.expectedHT) {
				t.Errorf("UnmarshalJSON() got struct = %+v, want %+v", ht, tt.expectedHT)
			}
		})
	}
}

func TestHumanTime_Value(t *testing.T) {
	localTimezone = time.FixedZone("Europe/Brussels", 2)
	defer func() { localTimezone = time.Local }()

	tests := []struct {
		name          string
		ht            HumanTime
		expectedValue driver.Value
		expectError   bool
	}{
		{
			name:          "Non-zero time",
			ht:            newHumanTimeLocal(2023, time.October, 27, 10, 30, 0),
			expectedValue: time.Date(2023, time.October, 27, 10, 30, 0, 0, localTimezone),
		},
		{
			name:          "Zero time",
			ht:            HumanTime{},
			expectedValue: time.Time{}, // Or potentially nil depending on driver interpretation of zero time{}
			// The code returns time.Time{}, which database/sql drivers often map to NULL or a specific zero time.
			// Let's test for time.Time{} as returned by the code.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.ht.Value()

			if tt.expectError {
				if err == nil {
					t.Errorf("Value() expected an error but got none")
				}

				return
			}

			if err != nil {
				t.Errorf("Value() unexpected error: %v", err)
				return
			}

			// Compare the database driver value
			// For time.Time comparison, use Equal method
			if expectedTime, ok := tt.expectedValue.(time.Time); ok {
				if actualTime, ok := value.(time.Time); ok {
					if !actualTime.Equal(expectedTime) {
						t.Errorf("Value() got time = %v, want %v", actualTime, expectedTime)
					}

					return
				}

				t.Errorf("Value() returned unexpected type %T, want time.Time", value)

				return
			}
			// For other types (like nil), use DeepEqual
			if !reflect.DeepEqual(value, tt.expectedValue) {
				t.Errorf("Value() got value = %v (%T), want %v (%T)", value, value, tt.expectedValue, tt.expectedValue)
			}
		})
	}
}

func TestHumanTime_Scan(t *testing.T) {
	localTimezone = time.FixedZone("Europe/Brussels", 2)
	defer func() { localTimezone = time.Local }()

	tests := []struct {
		name        string
		scanValue   interface{}
		expectedHT  HumanTime
		expectError bool
	}{
		{
			name:        "Scan nil (database NULL)",
			scanValue:   nil,
			expectedHT:  HumanTime{}, // Should be zero time
			expectError: false,
		},
		{
			name:        "Scan time.Time",
			scanValue:   time.Date(2023, time.October, 27, 10, 30, 0, 0, time.UTC), // Database drivers often return UTC
			expectedHT:  HumanTime{Time: time.Date(2023, time.October, 27, 10, 30, 0, 0, time.UTC)},
			expectError: false,
		},
		{
			name:        "Scan other type (e.g., string)",
			scanValue:   "2023-10-27",
			expectedHT:  HumanTime{}, // Should not modify receiver on error
			expectError: true,
		},
		{
			name:        "Scan zero time.Time",
			scanValue:   time.Time{},
			expectedHT:  HumanTime{Time: time.Time{}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ht HumanTime // Scan modifies the receiver
			err := ht.Scan(tt.scanValue)

			if tt.expectError {
				if err == nil {
					t.Errorf("Scan() expected an error but got none")
					return
				}
				// Optionally check error type/message
				expectedErrorSubstr := fmt.Sprintf("cannot scan type %T into HumanTime", tt.scanValue)
				if !strings.Contains(err.Error(), expectedErrorSubstr) {
					t.Errorf("Scan() error message got %q, want something containing %q", err.Error(), expectedErrorSubstr)
				}

				return
			}

			if err != nil {
				t.Errorf("Scan() unexpected error: %v", err)
				return
			}

			// Compare the scanned HumanTime
			// Use Equal for time comparison
			if !ht.Time.Equal(tt.expectedHT.Time) {
				t.Errorf("Scan() got time = %v, want %v", ht.Time, tt.expectedHT.Time)
			}

			// Check full struct equality too, especially for the zero value case
			if !reflect.DeepEqual(ht, tt.expectedHT) {
				t.Errorf("Scan() got struct = %+v, want %+v", ht, tt.expectedHT)
			}
		})
	}
}

func TestHumanTime_FormatDate(t *testing.T) {
	localTimezone = time.FixedZone("Europe/Brussels", 2)
	defer func() { localTimezone = time.Local }()

	tests := []struct {
		name     string
		ht       HumanTime
		expected string
	}{
		{
			name:     "Time with date and time part",
			ht:       newHumanTimeLocal(2023, time.October, 27, 10, 30, 15),
			expected: "2023-10-27 10:30",
		},
		{
			name:     "Time exactly at midnight",
			ht:       newHumanTimeLocal(2024, time.January, 1, 0, 0, 0),
			expected: "2024-01-01",
		},
		{
			name:     "Time with only hour part (0 minutes)",
			ht:       newHumanTimeLocal(2024, time.January, 1, 5, 0, 0),
			expected: "2024-01-01 05:00", // Hour != 0, so full format
		},
		{
			name:     "Time with only minute part (0 hours)",
			ht:       newHumanTimeLocal(2024, time.January, 1, 0, 10, 0),
			expected: "2024-01-01 00:10", // Minute != 0, so full format
		},
		{
			name:     "Zero time", // FormatDate doesn't special case zero time, depends on embedded Time's method
			ht:       HumanTime{Time: time.Time{}},
			expected: "0001-01-01", // Default zero time format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ht.FormatDate()
			if got != tt.expected {
				t.Errorf("FormatDate() got = %s, want %s", got, tt.expected)
			}
		})
	}
}

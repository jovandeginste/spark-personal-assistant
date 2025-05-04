package data

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntry_SetMetadata(t *testing.T) {
	tests := []struct {
		name          string
		initialMeta   map[string]any
		key           string
		value         any
		expectedMeta  map[string]any
		expectedNil   bool // Whether Metadata should be nil initially
		expectedCount int
	}{
		{
			name:          "Set when metadata is nil",
			initialMeta:   nil,
			key:           "newKey",
			value:         "newValue",
			expectedMeta:  map[string]any{"newKey": "newValue"},
			expectedNil:   true,
			expectedCount: 1,
		},
		{
			name:          "Set when metadata is not nil",
			initialMeta:   map[string]any{"existingKey": 123},
			key:           "newKey",
			value:         "newValue",
			expectedMeta:  map[string]any{"existingKey": 123, "newKey": "newValue"},
			expectedNil:   false,
			expectedCount: 2,
		},
		{
			name:          "Update existing key",
			initialMeta:   map[string]any{"existingKey": "oldValue"},
			key:           "existingKey",
			value:         "newValue",
			expectedMeta:  map[string]any{"existingKey": "newValue"},
			expectedNil:   false,
			expectedCount: 1,
		},
		{
			name:          "Set nil value",
			initialMeta:   map[string]any{"existingKey": "oldValue"},
			key:           "nilKey",
			value:         nil,
			expectedMeta:  map[string]any{"existingKey": "oldValue", "nilKey": nil},
			expectedNil:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Entry{Metadata: tt.initialMeta}

			e.SetMetadata(tt.key, tt.value)

			assert.NotNil(t, e.Metadata, "Metadata should not be nil after setting")
			assert.Equal(t, tt.expectedMeta, e.Metadata, "Metadata did not match expected")
			assert.Len(t, e.Metadata, tt.expectedCount, "Metadata count mismatch")
		})
	}
}

func TestEntry_GenerateRemoteID(t *testing.T) {
	tests := []struct {
		name             string
		initialRemoteID  string
		entry            Entry
		expectedRemoteID string
	}{
		{
			name:            "Generate when RemoteID is empty",
			initialRemoteID: "",
			entry: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Test summary",
			},
			// Calculate expected ID manually
			expectedRemoteID: generateHash(fmt.Sprintf("%d\n%s", time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone).UTC().Unix(), "Test summary")),
		},
		{
			name:            "Do not generate when RemoteID is already set",
			initialRemoteID: "existing-remote-id",
			entry: Entry{
				RemoteID: "existing-remote-id",
				Date:     HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary:  "Test summary",
			},
			expectedRemoteID: "existing-remote-id",
		},
		{
			name:            "Generate for different data",
			initialRemoteID: "",
			entry: Entry{
				Date:    HumanTime{time.Date(2023, 10, 27, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Another summary",
			},
			// Calculate expected ID manually
			expectedRemoteID: generateHash(fmt.Sprintf("%d\n%s", time.Date(2023, 10, 27, 12, 0, 0, 0, LocalTimezone).UTC().Unix(), "Another summary")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.entry
			e.RemoteID = tt.initialRemoteID // Explicitly set initial state

			e.GenerateRemoteID()

			assert.Equal(t, tt.expectedRemoteID, e.RemoteID, "RemoteID was not generated or should not have been changed")
		})
	}
}

func TestEntry_NewRemoteID(t *testing.T) {
	tests := []struct {
		name        string
		entry       Entry
		compare     Entry // Entry to compare ID with
		expectEqual bool  // Whether the IDs should be equal
	}{
		{
			name: "Same date, same summary",
			entry: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary A",
			},
			compare: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary A",
			},
			expectEqual: true,
		},
		{
			name: "Same date, different summary",
			entry: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary A",
			},
			compare: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary B",
			},
			expectEqual: false,
		},
		{
			name: "Different date, same summary",
			entry: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary A",
			},
			compare: Entry{
				Date:    HumanTime{time.Date(2023, 10, 27, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary A",
			},
			expectEqual: false,
		},
		{
			name: "Different date, different summary",
			entry: Entry{
				Date:    HumanTime{time.Date(2023, 10, 26, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary A",
			},
			compare: Entry{
				Date:    HumanTime{time.Date(2023, 10, 27, 12, 0, 0, 0, LocalTimezone)},
				Summary: "Summary B",
			},
			expectEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1 := tt.entry.NewRemoteID()
			id2 := tt.compare.NewRemoteID()

			assert.NotEmpty(t, id1, "Generated ID should not be empty")
			assert.NotEmpty(t, id2, "Generated ID should not be empty")

			if tt.expectEqual {
				assert.Equal(t, id1, id2, "IDs should be equal")
			} else {
				assert.NotEqual(t, id1, id2, "IDs should not be equal")
			}

			// Verify base64 encoding format
			decoded1, err := base64.URLEncoding.DecodeString(id1)
			assert.NoError(t, err, "Generated ID should be a valid URL-safe base64 string")
			assert.Len(t, decoded1, sha512.Size, "Decoded ID should have the size of SHA-512 hash")
		})
	}
}

func TestEntry_BeforeSave(t *testing.T) {
	// This test primarily checks if GenerateRemoteID is called.
	// Since GenerateRemoteID's behavior is tested separately,
	// we test the side effect: RemoteID should be set if it was empty.
	tests := []struct {
		name            string
		initialRemoteID string
		entry           Entry
		expectChange    bool
	}{
		{
			name:            "RemoteID is empty, should generate",
			initialRemoteID: "",
			entry: Entry{
				Date:    HumanTime{time.Now()},
				Summary: "Test summary for save",
			},
			expectChange: true,
		},
		{
			name:            "RemoteID is set, should not change",
			initialRemoteID: "existing-id-save",
			entry: Entry{
				RemoteID: "existing-id-save",
				Date:     HumanTime{time.Now()},
				Summary:  "Test summary for save",
			},
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.entry
			e.RemoteID = tt.initialRemoteID // Ensure initial state

			err := e.BeforeSave(nil) // Pass nil for DB as GenerateRemoteID doesn't use it

			assert.NoError(t, err)

			if tt.expectChange {
				assert.NotEmpty(t, e.RemoteID, "RemoteID should be set after BeforeSave when initially empty")
				assert.NotEqual(t, tt.initialRemoteID, e.RemoteID, "RemoteID should have changed")
			} else {
				assert.Equal(t, tt.initialRemoteID, e.RemoteID, "RemoteID should not change if already set")
			}
		})
	}
}

func TestEntry_AfterFind(t *testing.T) {
	// This test primarily checks if FormattedDate is called and assigned.
	// We need a HumanTime mock or structure that we can control the FormatDate output.
	// Using the simple HumanTime wrapper defined above which formats to "YYYY-MM-DD".
	testDate := time.Date(2023, 11, 15, 0, 0, 0, 0, LocalTimezone)
	expectedFormattedDate := "2023-11-15"

	e := &Entry{
		Date: HumanTime{testDate},
	}

	err := e.AfterFind(nil) // Pass nil for DB as FormattedDate doesn't use it

	assert.NoError(t, err)
	assert.Equal(t, expectedFormattedDate, e.DateString, "DateString was not populated correctly after AfterFind")
}

func TestEntry_FormattedDate(t *testing.T) {
	// This tests that FormattedDate calls the underlying Date.FormatDate method.
	// Using the simple HumanTime wrapper defined above which formats to "YYYY-MM-DD".
	testDate := time.Date(2024, 1, 1, 10, 30, 0, 0, LocalTimezone)
	expectedFormattedDate := "2024-01-01 10:30"

	e := &Entry{
		Date: HumanTime{testDate},
	}

	formattedDate := e.FormattedDate()

	assert.Equal(t, expectedFormattedDate, formattedDate, "FormattedDate did not return the expected format")
}

func Test_parseDate(t *testing.T) {
	tests := []struct {
		name          string
		dateString    string
		expectError   bool
		expectedYear  int
		expectedMonth time.Month
		expectedDay   int
		compareToday  bool // Special handling for ""
	}{
		{
			name:          "Valid date",
			dateString:    "2023-10-26",
			expectError:   false,
			expectedYear:  2023,
			expectedMonth: time.October,
			expectedDay:   26,
			compareToday:  false,
		},
		{
			name:         "Empty string",
			dateString:   "",
			expectError:  false,
			compareToday: true, // Expect today's date rounded
		},
		{
			name:         "Invalid format",
			dateString:   "10/26/2023",
			expectError:  true,
			compareToday: false,
		},
		{
			name:         "Invalid date",
			dateString:   "2023-02-30", // Feb has only 28/29 days
			expectError:  true,
			compareToday: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedTime, err := parseDate(tt.dateString)

			if tt.expectError {
				assert.Error(t, err, "Expected an error for invalid date string")
			} else {
				assert.NoError(t, err, "Did not expect an error for valid date string")

				if tt.compareToday {
					nowLocal := time.Now().In(LocalTimezone)
					// Round both times to the nearest day for comparison
					expectedTime := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, LocalTimezone)
					parsedTimeRounded := time.Date(parsedTime.Year(), parsedTime.Month(), parsedTime.Day(), 0, 0, 0, 0, LocalTimezone)

					assert.Equal(t, expectedTime, parsedTimeRounded, "Parsed date for empty string should be today's date in local timezone rounded")
				} else {
					assert.Equal(t, tt.expectedYear, parsedTime.Year(), "Year mismatch")
					assert.Equal(t, tt.expectedMonth, parsedTime.Month(), "Month mismatch")
					assert.Equal(t, tt.expectedDay, parsedTime.Day(), "Day mismatch")
					assert.Equal(t, LocalTimezone, parsedTime.Location(), "Time should be local")
					// Check time components are zeroed out for parsed dates
					assert.Equal(t, 0, parsedTime.Hour())
					assert.Equal(t, 0, parsedTime.Minute())
					assert.Equal(t, 0, parsedTime.Second())
					assert.Equal(t, 0, parsedTime.Nanosecond())
				}
			}
		})
	}
}

func TestEntry_SetDate(t *testing.T) {
	tests := []struct {
		name         string
		dateString   string
		expectError  bool
		expectedTime time.Time // Expected time if no error, for specific dates
		compareToday bool      // Whether to compare with today's date rounded
	}{
		{
			name:         "Valid date string",
			dateString:   "2024-01-15",
			expectError:  false,
			expectedTime: time.Date(2024, 1, 15, 0, 0, 0, 0, LocalTimezone),
			compareToday: false,
		},
		{
			name:         "Empty string",
			dateString:   "",
			expectError:  false,
			compareToday: true, // Expect today's date in local timezone rounded
		},
		{
			name:         "Invalid date string",
			dateString:   "invalid-date",
			expectError:  true,
			compareToday: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Entry{}
			err := e.SetDate(tt.dateString)

			if tt.expectError {
				assert.Error(t, err, "Expected an error for invalid date string")
			} else {
				assert.NoError(t, err, "Did not expect an error for valid date string")

				if tt.compareToday {
					nowLocal := time.Now().In(LocalTimezone)
					expectedTime := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, LocalTimezone)
					parsedTime := time.Date(e.Date.Year(), e.Date.Month(), e.Date.Day(), 0, 0, 0, 0, LocalTimezone)
					assert.Equal(t, expectedTime, parsedTime, "Set date for empty string should be today's date in local timezone rounded")
				} else {
					assert.Equal(t, tt.expectedTime, e.Date.Time, "Set date mismatch")
				}
			}
		})
	}
}

func TestEntry_SetImportance(t *testing.T) {
	tests := []struct {
		name               string
		importanceString   string
		expectError        bool
		expectedImportance Importance
		expectedErrText    string
	}{
		{
			name:               "Set LOW",
			importanceString:   "low",
			expectError:        false,
			expectedImportance: LOW,
		},
		{
			name:               "Set MEDIUM",
			importanceString:   "medium",
			expectError:        false,
			expectedImportance: MEDIUM,
		},
		{
			name:               "Set HIGH",
			importanceString:   "high",
			expectError:        false,
			expectedImportance: HIGH,
		},
		{
			name:               "Set invalid string",
			importanceString:   "critical",
			expectError:        true,
			expectedImportance: "", // Should remain zero value or previous value, but check error
			expectedErrText:    "invalid importance: critical",
		},
		{
			name:               "Set case-insensitive (should fail)", // Current implementation is case-sensitive
			importanceString:   "Low",
			expectError:        true,
			expectedImportance: "",
			expectedErrText:    "invalid importance: Low",
		},
		{
			name:               "Set empty string",
			importanceString:   "",
			expectError:        true,
			expectedImportance: "",
			expectedErrText:    "invalid importance: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Entry{} // Start with zero importance
			err := e.SetImportance(tt.importanceString)

			if tt.expectError {
				require.Error(t, err, "Expected an error for importance string '%s'", tt.importanceString)
				assert.ErrorContains(t, err, tt.expectedErrText, "Error message mismatch")
				assert.Equal(t, Importance(""), e.Importance, "Importance should not change on error") // Assuming zero value on error
			} else {
				assert.NoError(t, err, "Did not expect an error for importance string '%s'", tt.importanceString)
				assert.Equal(t, tt.expectedImportance, e.Importance, "Importance mismatch")
			}
		})
	}
}

func TestEntry_PrintTo(t *testing.T) {
	tests := []struct {
		name               string
		entry              Entry
		expectedSubstrings []string
	}{
		{
			name: "Basic entry",
			entry: Entry{
				ID:         123,
				RemoteID:   "remote-id-abc",
				Date:       HumanTime{time.Date(2023, 12, 10, 0, 0, 0, 0, LocalTimezone)},
				DateString: "2023-12-10", // Populated by AfterFind, used by PrintTo
				Importance: HIGH,
				Summary:    "A test summary",
				Source:     &Source{Name: "Test Source"},
			},
			expectedSubstrings: []string{
				"ID", "123",
				"Remote ID", "remote-id-abc",
				"Data", "2023-12-10",
				"Summary", "A test summary",
				"Importance", "high",
				"Source", "Test Source",
			},
		},
		{
			name: "Entry with metadata",
			entry: Entry{
				ID:         456,
				RemoteID:   "remote-id-def",
				Date:       HumanTime{time.Date(2024, 1, 5, 0, 0, 0, 0, LocalTimezone)},
				DateString: "2024-01-05",
				Importance: LOW,
				Summary:    "Another summary",
				Source:     &Source{Name: "Another Source"},
				Metadata: map[string]any{
					"key1": "value1",
					"key2": 99,
				},
			},
			expectedSubstrings: []string{
				"ID", "456",
				"Remote ID", "remote-id-def",
				"Data", "2024-01-05",
				"Summary", "Another summary",
				"Importance", "low",
				"Source", "Another Source",
				"key1", "value1",
				"key2", "99", // Metadata values are fmt.Sprintf("%v", v)
			},
		},
		{
			name: "Entry with nil source and no metadata",
			entry: Entry{
				ID:         789,
				RemoteID:   "remote-id-ghi",
				Date:       HumanTime{time.Date(2024, 2, 20, 0, 0, 0, 0, LocalTimezone)},
				DateString: "2024-02-20",
				Importance: MEDIUM,
				Summary:    "Summary with no source",
				Source:     nil, // Source is nil
				Metadata:   nil, // Metadata is nil
			},
			expectedSubstrings: []string{
				"ID", "789",
				"Remote ID", "remote-id-ghi",
				"Data", "2024-02-20",
				"Summary", "Summary with no source",
				"Importance", "medium",
			},
			// No metadata keys expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			e := tt.entry

			// Ensure DateString is set as PrintTo uses it
			_ = e.AfterFind(nil)

			e.PrintTo(&buf)

			output := buf.String()
			// fmt.Println(output) // Uncomment to see the generated table output

			// Check if all expected substrings are present.
			// This is a robust way to check table content without parsing the table format.
			for _, s := range tt.expectedSubstrings {
				assert.Contains(t, output, s, "Output should contain: %s", s)
			}

			// Specific check for Source Name if Source is nil
			if tt.entry.Source == nil {
				assert.NotContains(t, output, "Source")
			}

			// Check for metadata keys if metadata was present
			if tt.entry.Metadata != nil {
				for k := range tt.entry.Metadata {
					assert.Contains(t, output, k, "Output should contain metadata key: %s", k)
				}
			}
		})
	}
}

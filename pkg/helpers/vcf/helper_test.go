package vcf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awterman/monkey"
	_ "github.com/emersion/go-vcard" // Import needed for test data creation implicitly
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a temporary VCF file
func createTempVCFFile(t *testing.T, content string) string {
	tmpDir := t.TempDir() // Use t.TempDir() for automatic cleanup
	filePath := filepath.Join(tmpDir, "test.vcf")
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write temporary VCF file")
	return filePath
}

// TestBuildEntriesFromFile tests the BuildEntriesFromFile function using temporary files.
func TestBuildEntriesFromFile(t *testing.T) {
	// Mock time.Now() for deterministic parseBday calls within BuildEntriesFromFile
	fakeNow := time.Date(2024, time.March, 15, 12, 0, 0, 0, time.UTC)
	patchTime := monkey.Func(nil, time.Now, func() time.Time { return fakeNow })
	defer patchTime.Reset()

	type expectedEntry struct {
		Date     time.Time
		Summary  string
		Metadata map[string]any // Contains the Age field
	}

	tests := []struct {
		name            string
		vcfContent      string
		expectError     bool // Indicates if os.Open should fail (for non-existent file)
		expectedCount   int
		expectedEntries []expectedEntry
	}{
		{
			name: "Single valid vCard (YYYYMMDD)",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:John Doe
BDAY:19901026
END:VCARD`,
			expectedCount: 1,
			expectedEntries: []expectedEntry{
				{
					Date:     time.Date(2024, time.October, 26, 0, 0, 0, 0, time.UTC),
					Summary:  "Birthday John Doe",
					Metadata: map[string]any{"Age": 34},
				},
			},
		},
		{
			name: "Single valid vCard (--MMDD)",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:Jane Smith
BDAY:--1105
END:VCARD`,
			expectedCount: 1,
			expectedEntries: []expectedEntry{
				{
					Date:     time.Date(2024, time.November, 5, 0, 0, 0, 0, time.UTC),
					Summary:  "Birthday Jane Smith",
					Metadata: nil,
				},
			},
		},
		{
			name: "Multiple valid vCards",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:John Doe
BDAY:19901026
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Jane Smith
BDAY:--1105
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Another Person
BDAY:20000520
END:VCARD`,
			expectedCount: 3,
			expectedEntries: []expectedEntry{
				{Date: time.Date(2024, time.October, 26, 0, 0, 0, 0, time.UTC), Summary: "Birthday John Doe", Metadata: map[string]any{"Age": 34}},
				{Date: time.Date(2024, time.November, 5, 0, 0, 0, 0, time.UTC), Summary: "Birthday Jane Smith", Metadata: nil},
				{Date: time.Date(2024, time.May, 20, 0, 0, 0, 0, time.UTC), Summary: "Birthday Another Person", Metadata: map[string]any{"Age": 24}}, // 2024-2000
			},
		},
		{
			name: "vCard without BDAY",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:No Bday
END:VCARD`,
			expectedCount:   0, // Should skip this vCard
			expectedEntries: []expectedEntry{},
		},
		{
			name: "vCard with invalid BDAY format",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:Bad Bday
BDAY:26-10-1990 ; Invalid format
END:VCARD`,
			expectedCount:   0, // Should skip due to parseBday error
			expectedEntries: []expectedEntry{},
		},
		{
			name: "vCard without FN",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
BDAY:19901026
END:VCARD`,
			expectedCount:   0, // Should skip due to empty FN
			expectedEntries: []expectedEntry{},
		},
		{
			name: "Mix of valid and invalid vCards",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:Valid Person 1
BDAY:19850721
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Invalid Bday
BDAY:Not a date
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Valid Person 2
BDAY:--0430
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:No Bday FN set
END:VCARD`,
			expectedCount: 2, // Valid Person 1 and Valid Person 2
			expectedEntries: []expectedEntry{
				{Date: time.Date(2024, time.July, 21, 0, 0, 0, 0, time.UTC), Summary: "Birthday Valid Person 1", Metadata: map[string]any{"Age": 39}}, // 2024-1985
				{Date: time.Date(2024, time.April, 30, 0, 0, 0, 0, time.UTC), Summary: "Birthday Valid Person 2", Metadata: nil},                      // 2024-2024
			},
		},
		{
			name:            "Empty file",
			vcfContent:      "",
			expectedCount:   0,
			expectedEntries: []expectedEntry{},
		},
		{
			name:            "File with only whitespace",
			vcfContent:      "   \n\n",
			expectedCount:   0, // go-vcard should decode nothing or handle gracefully
			expectedEntries: []expectedEntry{},
		},
		{
			name: "Non-existent file",
			// No file content needed as os.Open will fail
			expectError:     true, // os.Open error
			expectedCount:   0,
			expectedEntries: []expectedEntry{},
		},
		{
			name: "vCard decoding error (simulated by invalid vcf)",
			vcfContent: `BEGIN:VCARD
VERSION:3.0
FN:Cause Decode Error
BDAY:19901026
X-INVALID-PROPERTY: value with invalid \\ escaping
INVALIDEND:VCARD`, // Malformed property might cause go-vcard.NewDecoder(f).Decode() to return an error other than io.EOF
			expectedCount:   0,
			expectError:     true, // os.Open error
			expectedEntries: []expectedEntry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if !tt.expectError { // Create file unless we expect an error related to file opening/decoding
				filePath = createTempVCFFile(t, tt.vcfContent)
			} else if tt.name == "Non-existent file" {
				// Use a path that definitely does not exist for the os.Open error test
				filePath = filepath.Join(t.TempDir(), "non", "existent", "path", "file.vcf")
			} else {
				// Create a temp file even if we expect a decode error,
				// as BuildEntriesFromFile still needs a file to open.
				filePath = createTempVCFFile(t, tt.vcfContent)
			}

			var entries data.Entries
			var err error

			// Call the function under test
			entries, err = BuildEntriesFromFile(filePath)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.name == "Non-existent file" {
					assert.True(t, errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file"), "Expected 'file not found' error")
				}
			} else { // If not expecting file error, expect success
				assert.NoError(t, err, "Did not expect an error")
				assert.Len(t, entries, tt.expectedCount, "Entry count mismatch")

				// Compare relevant fields of entries
				require.Equal(t, len(tt.expectedEntries), len(entries), "Mismatch in number of expected vs actual entries data for comparison")
				for i := range tt.expectedEntries {
					expected := tt.expectedEntries[i]
					actual := entries[i]

					// Compare Date (time.Time component of HumanTime)
					assert.True(t, expected.Date.Equal(actual.Date.Time), "Entry %d date mismatch: Expected %v, Got %v", i, expected.Date, actual.Date.Time)
					assert.Equal(t, expected.Summary, actual.Summary, "Entry %d summary mismatch", i)

					// Compare Metadata
					if expected.Metadata == nil {
						assert.Nil(t, actual.Metadata, "Entry %d Metadata should be nil", i)
					} else {
						assert.NotNil(t, actual.Metadata, "Entry %d Metadata should not be nil", i)
						// Perform a deep equality check on the metadata map
						assert.Equal(t, expected.Metadata, actual.Metadata, "Entry %d Metadata mismatch", i)
					}

					// Check that default/generated fields are not set by this function
					assert.Equal(t, uint64(0), actual.ID)
					assert.Equal(t, "", actual.RemoteID)
					assert.Equal(t, uint64(0), actual.SourceID)
					// DateString is populated AfterFind, not here
					assert.Nil(t, actual.Source)
				}
			}
		})
	}
}

// Note: The `collectAttendees` function is present in `ical/helper.go`, not `vcf/helper.go`.
// It is not included in the tests for this file as per the request.

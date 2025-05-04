package rss

import (
	"errors"
	"testing"
	"time"

	"github.com/awterman/monkey"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEntriesFromFeed(t *testing.T) {
	mockFeedURL := "http://example.com/feed.xml"

	tests := []struct {
		name               string
		feedURL            string
		mockFeed           *gofeed.Feed
		mockParseError     error
		expectError        bool
		expectedError      error
		expectedEntryCount int
		expectedEntries    []data.Entry // Check basic fields like Date.Time and Summary
		expectPanic        bool         // For the nil PublishedParsed case
	}{
		{
			name:    "Successful feed parsing",
			feedURL: mockFeedURL,
			mockFeed: &gofeed.Feed{
				Items: []*gofeed.Item{
					{
						Title:           "Item 1 Title",
						PublishedParsed: ptrTime(time.Date(2023, time.October, 26, 10, 0, 0, 0, time.UTC)),
						Link:            "http://example.com/item/1",
						Description:     "Desc 1",
					},
					{
						Title:           "Item 2 Title",
						PublishedParsed: ptrTime(time.Date(2023, time.October, 27, 12, 30, 0, 0, time.UTC)),
						Link:            "http://example.com/item/2",
						Description:     "Desc 2",
					},
				},
			},
			expectError:        false,
			expectedEntryCount: 2,
			expectedEntries: []data.Entry{
				{Date: data.HumanTime{Time: time.Date(2023, time.October, 26, 10, 0, 0, 0, time.UTC)}, Summary: "Item 1 Title"},
				{Date: data.HumanTime{Time: time.Date(2023, time.October, 27, 12, 30, 0, 0, time.UTC)}, Summary: "Item 2 Title"},
			},
		},
		{
			name:               "Empty feed items",
			feedURL:            mockFeedURL,
			mockFeed:           &gofeed.Feed{Items: []*gofeed.Item{}},
			expectError:        true,
			expectedError:      errors.New("no events"),
			expectedEntryCount: 0,
		},
		{
			name:               "nil feed items slice", // Should be treated same as empty
			feedURL:            mockFeedURL,
			mockFeed:           &gofeed.Feed{Items: nil},
			expectError:        true,
			expectedError:      errors.New("no events"),
			expectedEntryCount: 0,
		},
		{
			name:               "Error from ParseURL",
			feedURL:            mockFeedURL,
			mockFeed:           nil, // Feed should be nil on error
			mockParseError:     errors.New("network error"),
			expectError:        true,
			expectedError:      errors.New("network error"),
			expectedEntryCount: 0,
		},
		{
			name:    "Feed item with nil PublishedParsed", // Should cause panic in current code
			feedURL: mockFeedURL,
			mockFeed: &gofeed.Feed{
				Items: []*gofeed.Item{
					{
						Title:           "Item with no date",
						PublishedParsed: nil, // Explicitly nil
						Link:            "http://example.com/item/nodefdate",
					},
				},
			},
			expectError:        false, // The panic happens before error handling
			expectPanic:        true,
			expectedEntryCount: 0, // Entries slice is built before loop, but panic occurs inside
		},
		{
			name:    "Feed item with empty Title",
			feedURL: mockFeedURL,
			mockFeed: &gofeed.Feed{
				Items: []*gofeed.Item{
					{
						Title:           "", // Empty title
						PublishedParsed: ptrTime(time.Date(2023, time.November, 1, 8, 0, 0, 0, time.UTC)),
					},
				},
			},
			expectError:        false,
			expectedEntryCount: 1,
			expectedEntries: []data.Entry{
				{Date: data.HumanTime{Time: time.Date(2023, time.November, 1, 8, 0, 0, 0, time.UTC)}, Summary: ""}, // Summary should be empty
			},
		},
	}

	for _, tt := range tests {
		// Set the mock behavior for ParseURL within this test's closure
		// Setup patching before running tests and reset after
		patch := monkey.Method(nil, fp, fp.ParseURL,
			func(url string) (*gofeed.Feed, error) {
				return tt.mockFeed, tt.mockParseError
			},
		)
		defer patch.Reset()

		t.Run(tt.name, func(t *testing.T) {
			// Use defer recover to catch expected panics
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Fatalf("Test panicked unexpectedly: %v", r)
					}
					// Check if the panic is the expected nil pointer dereference
					assert.Contains(t, r, "nil pointer dereference", "Panic message did not indicate nil pointer dereference")
				} else if tt.expectPanic {
					t.Fatalf("Test did not panic as expected")
				}
			}()

			// Call the function under test
			entries, err := BuildEntriesFromFeed(tt.feedURL)

			// Check for errors (if not expecting panic)
			if !tt.expectPanic {
				if tt.expectError {
					assert.Error(t, err, "Expected an error")

					if tt.expectedError != nil {
						assert.Equal(t, tt.expectedError, err, "Error mismatch")
					}

					assert.Nil(t, entries, "Expected nil entries on error")
				} else {
					assert.NoError(t, err, "Did not expect an error")
					require.NotNil(t, entries, "Expected non-nil entries on success")
					assert.Len(t, entries, tt.expectedEntryCount, "Entry count mismatch")

					// Verify basic fields for expected entries
					require.Equal(t, len(tt.expectedEntries), len(entries), "Mismatch in number of expected vs actual entries for detailed check")

					for i := range tt.expectedEntries {
						expected := tt.expectedEntries[i]
						actual := entries[i]

						// Compare time.Time component of HumanTime
						assert.True(t, expected.Date.Time.Equal(actual.Date.Time), "Entry %d date mismatch: Expected %v, Got %v", i, expected.Date.Time, actual.Date.Time)
						assert.Equal(t, expected.Summary, actual.Summary, "Entry %d summary mismatch", i)

						// Ensure other fields are default/zero values as they are not populated by this function
						assert.Equal(t, uint64(0), actual.ID)
						assert.Equal(t, "", actual.RemoteID)
						assert.Equal(t, data.Importance(""), actual.Importance) // Or data.MEDIUM based on struct default? Check Entry struct default. It's not set by this function.
						assert.Equal(t, uint64(0), actual.SourceID)
						assert.Nil(t, actual.Metadata)
						assert.Equal(t, "", actual.DateString) // Populated by AfterFind, not here
						assert.Nil(t, actual.Source)
					}
				}
			}
		})
	}
}

// Helper function to get a pointer to a time.Time value
func ptrTime(t time.Time) *time.Time {
	return &t
}

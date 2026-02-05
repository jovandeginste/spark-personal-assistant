package vcf

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseVCF(t *testing.T) {
	vcfContent := `BEGIN:VCARD
VERSION:3.0
FN:John Doe
BDAY:19900101
EMAIL:john@example.com
ADR;TYPE=HOME:;;123 Main St.;Anytown;CA;12345;USA
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Jane Doe
BDAY;VALUE=DATE:1995-05-15
EMAIL:jane@example.com
EMAIL:jane.doe@work.com
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:No Birthday
END:VCARD`

	reader := strings.NewReader(vcfContent)
	contacts, err := parseVCF(reader)

	assert.NoError(t, err)
	assert.Len(t, contacts, 3)

	assert.Equal(t, "John Doe", contacts[0].Name)
	assert.Equal(t, "19900101", contacts[0].Birthday)
	assert.Equal(t, 1990, contacts[0].BirthDate.Year)
	assert.Equal(t, 1, contacts[0].BirthDate.Month)
	assert.Equal(t, 1, contacts[0].BirthDate.Day)
	assert.Len(t, contacts[0].Emails, 1)
	assert.Equal(t, "john@example.com", contacts[0].Emails[0])
	assert.Len(t, contacts[0].Addresses, 1)
	assert.Equal(t, "123 Main St.", contacts[0].Addresses[0].Street)
	assert.Equal(t, "Anytown", contacts[0].Addresses[0].Locality)
	assert.Equal(t, "CA", contacts[0].Addresses[0].Region)
	assert.Equal(t, "12345", contacts[0].Addresses[0].PostalCode)
	assert.Equal(t, "USA", contacts[0].Addresses[0].Country)
	assert.Equal(t, "123 Main St., Anytown, CA, 12345, USA", contacts[0].Addresses[0].FullAddress)

	assert.Equal(t, "Jane Doe", contacts[1].Name)
	assert.Equal(t, "1995-05-15", contacts[1].Birthday)
	assert.Equal(t, 1995, contacts[1].BirthDate.Year)
	assert.Equal(t, 5, contacts[1].BirthDate.Month)
	assert.Equal(t, 15, contacts[1].BirthDate.Day)
	assert.Len(t, contacts[1].Emails, 2)
	assert.Contains(t, contacts[1].Emails, "jane@example.com")
	assert.Contains(t, contacts[1].Emails, "jane.doe@work.com")

	assert.Equal(t, "No Birthday", contacts[2].Name)
	assert.Empty(t, contacts[2].Birthday)
	assert.Nil(t, contacts[2].BirthDate)
	assert.Empty(t, contacts[2].Emails)
}

func TestFindBirthdays(t *testing.T) {
	contacts := []Contact{
		{
			Name:      "Jan First",
			BirthDate: &Date{Year: 1990, Month: 1, Day: 1},
		},
		{
			Name:      "Jan Second",
			BirthDate: &Date{Year: 1990, Month: 1, Day: 2},
		},
		{
			Name:      "Feb First",
			BirthDate: &Date{Year: 1990, Month: 2, Day: 1},
		},
		{
			Name:      "Dec Last",
			BirthDate: &Date{Year: 1990, Month: 12, Day: 31},
		},
	}

	// Test exact date
	results := findBirthdays(contacts, Date{Month: 1, Day: 1}, Date{Month: 1, Day: 1})
	assert.Len(t, results, 1)
	assert.Equal(t, "Jan First", results[0].Name)

	// Test range within month
	results = findBirthdays(contacts, Date{Month: 1, Day: 1}, Date{Month: 1, Day: 2})
	assert.Len(t, results, 2)

	// Test range across months
	results = findBirthdays(contacts, Date{Month: 1, Day: 1}, Date{Month: 2, Day: 1})
	assert.Len(t, results, 3)

	// Test year wrap-around (Dec 31 to Jan 2)
	results = findBirthdays(contacts, Date{Month: 12, Day: 31}, Date{Month: 1, Day: 2})
	assert.Len(t, results, 3) // Dec Last, Jan First, Jan Second

	// Check content of wrap around result
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "Dec Last")
	assert.Contains(t, names, "Jan First")
	assert.Contains(t, names, "Jan Second")
}

func TestAgeCalculation(t *testing.T) {
	currentYear := time.Now().Year()

	contacts := []Contact{
		{
			Name:      "Born 1990",
			BirthDate: &Date{Year: 1990, Month: 1, Day: 1},
		},
		{
			Name:      "Born 2000",
			BirthDate: &Date{Year: 2000, Month: 1, Day: 1},
		},
		{
			Name:      "No Year",
			BirthDate: &Date{Year: 0, Month: 1, Day: 1},
		},
	}

	results := findBirthdays(contacts, Date{Month: 1, Day: 1}, Date{Month: 1, Day: 1})
	assert.Len(t, results, 3)

	for _, c := range results {
		switch c.Name {
		case "Born 1990":
			assert.Equal(t, currentYear-1990, c.Age)
		case "Born 2000":
			assert.Equal(t, currentYear-2000, c.Age)
		case "No Year":
			assert.Equal(t, 0, c.Age)
		}
	}
}

func TestParseBirthday(t *testing.T) {
	tests := []struct {
		input    string
		expected *Date
		wantErr  bool
	}{
		{"19900101", &Date{Year: 1990, Month: 1, Day: 1}, false},
		{"1990-01-01", &Date{Year: 1990, Month: 1, Day: 1}, false},
		{"invalid", nil, true},
		{"123", nil, true},
	}

	for _, tt := range tests {
		result, err := parseBirthday(tt.input)
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		}
	}
}

func TestIntegration(t *testing.T) {
	// This is just to ensure the package builds and public interface is correct
	// Real integration would require file system access

	// Simulate current date for "today" test
	now := time.Now()
	today := Date{Month: int(now.Month()), Day: now.Day()}

	contacts := []Contact{
		{Name: "Today Birthday", BirthDate: &today},
	}

	results := findBirthdays(contacts, today, today)
	assert.Len(t, results, 1)
	assert.Equal(t, "Today Birthday", results[0].Name)
}

func TestFindContactByName(t *testing.T) {
	contacts := []Contact{
		{Name: "John Doe", Emails: []string{"john@example.com"}, Addresses: []Address{{FullAddress: "123 Main St., Anytown, CA, 12345, USA"}}},
		{Name: "Jane Smith", Emails: []string{"jane@example.com", "jsmith@work.com"}},
		{Name: "Bob Johnson"},
	}

	tests := []struct {
		query    string
		expected int
	}{
		{"John", 2},         // Matches John Doe (name) and Bob Johnson (name contains "ohn")
		{"doe", 1},          // Matches John Doe (name)
		{"Smith", 1},        // Matches Jane Smith (name)
		{"example.com", 2},  // Matches John and Jane (email)
		{"jsmith", 1},       // Matches Jane (email)
		{"alice", 0},        // No match
		{"Main St", 1},      // Matches John Doe (address)
		{"Anytown", 1},      // Matches John Doe (address)
		{"USA", 1},          // Matches John Doe (address)
		{"anytown", 1},      // Matches John Doe (address) (case insensitive)
		{"usa", 1},          // Matches John Doe (address) (case insensitive)
		{"town", 1},         // Matches John Doe (partial city "Anytown")
		{"US", 1},           // Matches John Doe (partial country "USA")
		{"", 3},             // Matches all
	}

	for _, tt := range tests {
		results := findContactByName(contacts, tt.query)
		assert.Len(t, results, tt.expected, "Query: %s", tt.query)
	}
}

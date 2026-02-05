package vcf

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
)

type Contact struct {
	Name      string         `json:"name"`
	Birthday  string         `json:"birthday,omitempty"`
	BirthDate *Date          `json:"birth_date,omitempty"`
	Age       int            `json:"age,omitempty"`
	Emails    []string       `json:"emails,omitempty"`
	Addresses []Address      `json:"addresses,omitempty"`
	Extras    map[string]any `json:"extras,omitempty"`
}

type Address struct {
	Street      string `json:"street,omitempty"`
	Locality    string `json:"locality,omitempty"` // City
	Region      string `json:"region,omitempty"`   // State/Province
	PostalCode  string `json:"postal_code,omitempty"`
	Country     string `json:"country,omitempty"`
	FullAddress string `json:"full_address,omitempty"`
}

type Date struct {
	Year  int
	Month int
	Day   int
}

func loadContacts(path string, _ caching.Cache) ([]Contact, error) {
	// We'll use the cache to store the parsed contacts if possible,
	// or just read the file directly since VCFs are usually local files.
	// For simplicity, let's just read the file directly for now,
	// but we could use the cache service if we were fetching from a URL.

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseVCF(f)
}

func parseVCF(r io.Reader) ([]Contact, error) {
	dec := vcard.NewDecoder(r)

	var contacts []Contact
	for {
		card, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		c := Contact{
			Extras: make(map[string]any),
		}

		c.parseName(card)
		c.parseBirthday(card)
		c.parseEmails(card)
		c.parseAddresses(card)
		c.parseExtras(card)

		contacts = append(contacts, c)
	}
	return contacts, nil
}

func (c *Contact) parseName(card vcard.Card) {
	c.Name = card.PreferredValue(vcard.FieldFormattedName)
	if c.Name == "" {
		// Fallback to Name field components if FN is missing
		name := card.Name()
		if name != nil {
			c.Name = strings.TrimSpace(name.GivenName + " " + name.FamilyName)
		}
	}
}

func (c *Contact) parseBirthday(card vcard.Card) {
	bday := card.PreferredValue(vcard.FieldBirthday)
	if bday != "" {
		c.Birthday = bday
		if bd, err := parseBirthday(bday); err == nil {
			c.BirthDate = bd
		}
	}
}

func (c *Contact) parseEmails(card vcard.Card) {
	for _, email := range card[vcard.FieldEmail] {
		c.Emails = append(c.Emails, email.Value)
	}
}

func (c *Contact) parseAddresses(card vcard.Card) {
	for _, adr := range card.Addresses() {
		a := Address{
			Street:     adr.StreetAddress,
			Locality:   adr.Locality,
			Region:     adr.Region,
			PostalCode: adr.PostalCode,
			Country:    adr.Country,
		}
		// Construct full address string for easier searching
		var parts []string
		if a.Street != "" {
			parts = append(parts, a.Street)
		}
		if a.Locality != "" {
			parts = append(parts, a.Locality)
		}
		if a.Region != "" {
			parts = append(parts, a.Region)
		}
		if a.PostalCode != "" {
			parts = append(parts, a.PostalCode)
		}
		if a.Country != "" {
			parts = append(parts, a.Country)
		}
		a.FullAddress = strings.Join(parts, ", ")

		if adr.ExtendedAddress != "" {
			a.FullAddress += ", " + adr.ExtendedAddress
		}
		c.Addresses = append(c.Addresses, a)
	}
}

func (c *Contact) parseExtras(card vcard.Card) {
	// Store all fields in Extras
	for k, fields := range card {
		if k == vcard.FieldFormattedName || k == vcard.FieldBirthday || k == vcard.FieldEmail || k == vcard.FieldAddress {
			continue
		}
		// Simplified representation for JSON
		values := make([]string, 0, len(fields))
		for _, f := range fields {
			values = append(values, f.Value)
		}
		if len(values) == 1 {
			c.Extras[k] = values[0]
		} else if len(values) > 1 {
			c.Extras[k] = values
		}
	}
}

func parseBirthday(s string) (*Date, error) {
	// Simple parsing for YYYYMMDD or --MMDD or YYYY-MM-DD
	s = strings.TrimSpace(s)
	// Try standard parsing first if it matches specific lengths
	if len(s) == 8 {
		// YYYYMMDD
		y, err := strconv.Atoi(s[0:4])
		if err != nil {
			return nil, err
		}
		m, err := strconv.Atoi(s[4:6])
		if err != nil {
			return nil, err
		}
		d, err := strconv.Atoi(s[6:8])
		if err != nil {
			return nil, err
		}
		return &Date{Year: y, Month: m, Day: d}, nil
	} else if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		// YYYY-MM-DD
		y, err := strconv.Atoi(s[0:4])
		if err != nil {
			return nil, err
		}
		m, err := strconv.Atoi(s[5:7])
		if err != nil {
			return nil, err
		}
		d, err := strconv.Atoi(s[8:10])
		if err != nil {
			return nil, err
		}
		return &Date{Year: y, Month: m, Day: d}, nil
	}
	// Attempt to parse with time.Parse for other ISO8601 variants commonly found in vCards
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}, nil
	}
	if t, err := time.Parse("20060102", s); err == nil {
		return &Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}, nil
	}

	return nil, errors.New("unknown date format")
}

func parseDate(s string) (Date, error) {
	// Expect MM-DD
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return Date{}, errors.New("invalid format, expected MM-DD")
	}
	m, err := strconv.Atoi(parts[0])
	if err != nil {
		return Date{}, err
	}
	d, err := strconv.Atoi(parts[1])
	if err != nil {
		return Date{}, err
	}
	return Date{Month: m, Day: d}, nil
}

func checkDateRange(bd *Date, start, end Date) bool {
	if start.Month > end.Month || (start.Month == end.Month && start.Day > end.Day) {
		// Range crosses year boundary
		if (bd.Month > start.Month || (bd.Month == start.Month && bd.Day >= start.Day)) ||
			(bd.Month < end.Month || (bd.Month == end.Month && bd.Day <= end.Day)) {
			return true
		}
	} else {
		// Normal range
		if (bd.Month > start.Month || (bd.Month == start.Month && bd.Day >= start.Day)) &&
			(bd.Month < end.Month || (bd.Month == end.Month && bd.Day <= end.Day)) {
			return true
		}
	}
	return false
}

func findBirthdays(contacts []Contact, start, end Date) []Contact {
	var results []Contact
	now := time.Now()
	currentYear := now.Year()

	for _, c := range contacts {
		if c.BirthDate == nil {
			continue
		}

		if checkDateRange(c.BirthDate, start, end) {
			// Calculate age if year is present
			if c.BirthDate.Year > 0 {
				c.Age = currentYear - c.BirthDate.Year
			}
			results = append(results, c)
		}
	}
	return results
}

func findContactByName(contacts []Contact, query string) []Contact {
	var results []Contact
	query = strings.ToLower(query)
	for _, c := range contacts {
		// Search name
		if strings.Contains(strings.ToLower(c.Name), query) {
			results = append(results, c)
			continue
		}
		// Search emails
		matchFound := false
		for _, email := range c.Emails {
			if strings.Contains(strings.ToLower(email), query) {
				results = append(results, c)
				matchFound = true
				break
			}
		}
		if matchFound {
			continue
		}
		// Search addresses
		for _, addr := range c.Addresses {
			if strings.Contains(strings.ToLower(addr.FullAddress), query) {
				results = append(results, c)
				break
			}
		}
	}
	return results
}

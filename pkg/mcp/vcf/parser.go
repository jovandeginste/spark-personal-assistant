package vcf

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
)

type Contact struct {
	Name      string `json:"name"`
	Birthday  string `json:"birthday,omitempty"` // stored as YYYYMMDD or --MMDD
	BirthDate *Date  `json:"birth_date,omitempty"`
	Age       int    `json:"age,omitempty"`
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
	scanner := bufio.NewScanner(r)
	var contacts []Contact
	var current *Contact

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "BEGIN:VCARD"):
			current = &Contact{}
		case strings.HasPrefix(line, "END:VCARD"):
			if current != nil && (current.Name != "" || current.Birthday != "") {
				contacts = append(contacts, *current)
			}
			current = nil
		case current != nil:
			if strings.HasPrefix(line, "FN:") {
				current.Name = strings.TrimPrefix(line, "FN:")
			} else if strings.HasPrefix(line, "BDAY") {
				// Format can be BDAY:19900101 or BDAY;VALUE=DATE:19900101
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					bday := parts[len(parts)-1]
					current.Birthday = bday
					if d, err := parseBirthday(bday); err == nil {
						current.BirthDate = d
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return contacts, nil
}

func parseBirthday(s string) (*Date, error) {
	// Simple parsing for YYYYMMDD or --MMDD
	s = strings.TrimSpace(s)
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

func findContactByName(contacts []Contact, name string) []Contact {
	var results []Contact
	name = strings.ToLower(name)
	for _, c := range contacts {
		if strings.Contains(strings.ToLower(c.Name), name) {
			results = append(results, c)
		}
	}
	return results
}

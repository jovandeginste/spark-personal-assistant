package vcf

import (
	"bufio"
	"fmt"
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

func loadContacts(path string, cache caching.Cache) ([]Contact, error) {
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
		
		if strings.HasPrefix(line, "BEGIN:VCARD") {
			current = &Contact{}
		} else if strings.HasPrefix(line, "END:VCARD") {
			if current != nil && (current.Name != "" || current.Birthday != "") {
				contacts = append(contacts, *current)
			}
			current = nil
		} else if current != nil {
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

	return contacts, nil
}

func parseBirthday(s string) (*Date, error) {
	// Simple parsing for YYYYMMDD or --MMDD
	s = strings.TrimSpace(s)
	if len(s) == 8 {
		// YYYYMMDD
		y, _ := strconv.Atoi(s[0:4])
		m, _ := strconv.Atoi(s[4:6])
		d, _ := strconv.Atoi(s[6:8])
		return &Date{Year: y, Month: m, Day: d}, nil
	} else if len(s) == 10 && s[4] == '-' && s[7] == '-' {
        // YYYY-MM-DD
        y, _ := strconv.Atoi(s[0:4])
        m, _ := strconv.Atoi(s[5:7])
        d, _ := strconv.Atoi(s[8:10])
        return &Date{Year: y, Month: m, Day: d}, nil
    }
	return nil, fmt.Errorf("unknown date format")
}

func parseDate(s string) (Date, error) {
	// Expect MM-DD
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return Date{}, fmt.Errorf("invalid format, expected MM-DD")
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

func findBirthdays(contacts []Contact, start, end Date) []Contact {
	var results []Contact
	now := time.Now()
	currentYear := now.Year()

	for _, c := range contacts {
		if c.BirthDate == nil {
			continue
		}

		// Check if birthday falls within range (ignoring year)
		bd := c.BirthDate
		
		match := false
		// Handle wrap-around year (e.g. Dec 25 to Jan 5)
		if start.Month > end.Month || (start.Month == end.Month && start.Day > end.Day) {
			// Range crosses year boundary
			if (bd.Month > start.Month || (bd.Month == start.Month && bd.Day >= start.Day)) ||
			   (bd.Month < end.Month || (bd.Month == end.Month && bd.Day <= end.Day)) {
				match = true
			}
		} else {
			// Normal range
			if (bd.Month > start.Month || (bd.Month == start.Month && bd.Day >= start.Day)) &&
			   (bd.Month < end.Month || (bd.Month == end.Month && bd.Day <= end.Day)) {
				match = true
			}
		}

		if match {
			// Calculate age if year is present
			if bd.Year > 0 {
				c.Age = currentYear - bd.Year
				// If birthday hasn't happened yet this year, subtract 1
				// Wait, if we are looking for upcoming birthdays, we probably want the age they *will* turn?
				// The prompt says "add the age of the contact". Usually means current age.
				// However, if I list "Upcoming birthdays", showing "turning 30" is more useful than "is 29".
				// Let's stick to "age they are turning this year" effectively, which is just currentYear - bd.Year
				// actually, technically if today is Jan 1 and birthday is Dec 31, they are not that age yet.
				// But usually when listing birthdays people want to know "John is turning 30".
				
				// Let's check if the birthday has already passed THIS YEAR relative to "now".
				// But the request might be for a different date range.
				// Let's keep it simple: Age = currentYear - bd.Year (the age they reach in the current calendar year).
				// Or be more precise: Age = currentYear - bd.Year. If (now < birthday_this_year) Age-- ?
				
				// If I assume the user wants to know how old they are *right now*, I should check today's date.
				// If I assume the user wants to know how old they will be *on their birthday*, it is simply year - birthyear.
				// Given this is a "birthday list" tool, "turning X" is the most common interpretation.
				// So I will provide Age = CurrentYear - BirthYear.
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

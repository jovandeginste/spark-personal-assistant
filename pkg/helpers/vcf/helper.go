package vcf

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
)

func BuildEntriesFromFile(file string) (data.Entries, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	dec := vcard.NewDecoder(f)

	var entries data.Entries

	for {
		card, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		name := card.PreferredValue(vcard.FieldFormattedName)
		if name == "" {
			continue
		}

		bday, age, err := parseBday(card.PreferredValue(vcard.FieldBirthday))
		if err != nil {
			continue
		}

		e := data.Entry{
			Date:    data.HumanTime{Time: bday},
			Summary: "Birthday " + name,
		}
		if age > 0 {
			e.SetMetadata("Age", age)
		}

		entries = append(entries, e)
	}

	return entries, nil
}

func parseBday(bday string) (time.Time, int, error) {
	if bday == "" {
		return time.Time{}, 0, errors.New("no birthday")
	}

	if strings.HasPrefix(bday, "--") {
		bday = fmt.Sprintf("%d%s", time.Now().Year(), strings.TrimPrefix(bday, "--"))
	}

	bdayDate, err := time.Parse("20060102", bday)
	if err != nil {
		return time.Time{}, 0, err
	}

	age := time.Now().Year() - bdayDate.Year()
	bdayDate = bdayDate.AddDate(age, 0, 0)

	return bdayDate, age, nil
}

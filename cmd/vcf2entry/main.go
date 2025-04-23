package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
)

func main() {
	if len(os.Args) < 2 {
		log.Println("Usage:", os.Args[0], "<file.vcf>")
		return
	}

	file := os.Args[1]

	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	dec := vcard.NewDecoder(f)

	var results data.Entries

	for {
		card, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
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

		results = append(results, e)
	}

	out, err := json.Marshal(results)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
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

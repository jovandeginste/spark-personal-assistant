package recycleapp

import (
	"log/slog"
	"time"

	"github.com/jordic/goics"
)

const collectionType = "collection"

type Organization struct {
	Name        string            `json:"name"`
	URL         map[string]string `json:"url"`
	Description map[string]string `json:"description"`
}

func (r *Organization) URLForLanguage(lang string) string {
	if u, ok := r.URL[lang]; ok {
		return u
	}

	return "???"
}

type StreetResponse struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
	Total int `json:"total"`
	Pages int `json:"pages"`
	Page  int `json:"page"`
	Size  int `json:"size"`
}

type ZipResponse struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
	Total int `json:"total"`
	Pages int `json:"pages"`
	Page  int `json:"page"`
	Size  int `json:"size"`
}

type RecycleInfo struct {
	Items []RecycleItem `json:"items"`
	Page  int           `json:"page"`
	Total int           `json:"total"`
	Size  int           `json:"size"`
	Self  string        `json:"self"`
	Last  string        `json:"last"`
	First string        `json:"first"`

	Org  *Organization `json:"org,omitempty"`
	Lang string        `json:"-"`
}

type JSONEvent struct {
	Summary string `json:"summary"`
	Date    string `json:"date"`
	Color   string `json:"color"`
}

func (r RecycleInfo) ToJSONEvents() []JSONEvent {
	events := []JSONEvent{}
	for _, i := range r.Items {
		if !i.IsCollection() {
			continue
		}
		events = append(events, JSONEvent{
			Summary: i.FractionName(r.Lang),
			Date:    i.Timestamp.Format("2006-01-02"),
			Color:   i.Fraction.Color,
		})
	}
	return events
}

type RecycleItem struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Fraction  RecycleFraction  `json:"fraction"`
	Exception RecycleException `json:"exception"`
}

type RecycleFraction struct {
	Name      map[string]string `json:"name"`
	Color     string            `json:"color"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type RecycleException struct {
	ReplacedBy struct {
		Type string `json:"type"`
	} `json:"replacedBy"`
	Replaces struct {
		Type string `json:"type"`
	} `json:"replaces"`
}

func (r *RecycleItem) IsCollection() bool {
	if r.Type != collectionType {
		return false
	}

	return r.Exception.IsCollection()
}

func (r *RecycleItem) FractionName(lang string) string {
	if name, ok := r.Fraction.Name[lang]; ok {
		return name
	}

	return "???"
}

func (r *RecycleException) Type() string {
	if r.ReplacedBy.Type == collectionType {
		return "replaced_by"
	}

	if r.Replaces.Type == collectionType {
		return "replaces"
	}

	return "normal"
}

func (r *RecycleException) IsCollection() bool {
	return r.Type() != "replaced_by"
}

func (r RecycleInfo) EmitICal() goics.Componenter {
	slog.Info("Emitting calendar", "item_count", len(r.Items))

	cal := goics.NewComponent()

	cal.SetType("VCALENDAR")
	cal.AddProperty("VERSION", "2.0")
	cal.AddProperty("PRODID", "-//Recycling calendar")
	cal.AddProperty("METHOD", "REQUEST")

	for _, i := range r.Items {
		e := i.ToEvent(r.Org, r.Lang)
		if e != nil {
			cal.AddComponent(e)
		}
	}

	return cal
}

func (r *RecycleItem) ToEvent(org *Organization, lang string) goics.Componenter {
	if !r.IsCollection() {
		return nil
	}

	s := goics.NewComponent()
	s.SetType("VEVENT")
	s.AddProperty("UID", r.ID)

	AddDateTimeField(s, "LAST-MODIFIED", r.Fraction.UpdatedAt)
	AddDateTimeField(s, "CREATED", r.Fraction.CreatedAt)
	AddDateTimeField(s, "DTSTAMP", r.Timestamp)

	k, v := goics.FormatDateField("DTSTART", r.Timestamp)
	s.AddProperty(k, v)

	s.AddProperty("COLOR", r.Fraction.Color)
	s.AddProperty("TRANSP", "TRANSPARENT")
	s.AddProperty("SUMMARY", r.FractionName(lang))
	if org != nil {
		s.AddProperty("ORGANIZER;CN="+org.Name, "nomail")
	} else {
		s.AddProperty("ORGANIZER;CN=Unknown", "nomail")
	}
	s.AddProperty("TZID", "Europe/Brussels")

	s.AddComponent(r.Alarm(lang))

	return s
}

func (r *RecycleItem) Alarm(lang string) goics.Componenter {
	s := goics.NewComponent()

	s.SetType("VALARM")
	s.AddProperty("TRIGGER;RELATED=START", "-PT6H")
	s.AddProperty("ACTION", "DISPLAY")
	s.AddProperty("DESCRIPTION", r.FractionName(lang))

	return s
}

func AddDateTimeField(cal goics.Componenter, name string, t time.Time) {
	k, v := goics.FormatDateTime(name, t)
	cal.AddProperty(k, v)
}

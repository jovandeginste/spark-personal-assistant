package twizzit

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"golang.org/x/net/html"
)

type ActivityDetails struct {
	Contact     Contact      `json:"contact"`
	Activity    Activity     `json:"activity"`
	Attendances []Attendance `json:"attendances"`
}

type Contact struct {
	ID        int    `json:"id"`
	FirstName string `json:"firstName"`
	Name      string `json:"name"`
	Image     string `json:"image"`
}

type Activity struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	DateString    string `json:"dateString"`
	StartDateTime string `json:"startDateTime"`
	MeetingTime   string `json:"meetingTime"`
	Description   string `json:"description"`
}

type Attendance struct {
	AttendanceTypeName string `json:"attendanceTypeName"`
	FirstName          string `json:"firstName,omitempty"`
	Name               string `json:"name,omitempty"`
	Comment            string `json:"comment,omitempty"`
	Order              int    `json:"-"`
}

func parseActivityDetails(htmlContent string) (*ActivityDetails, error) {
	// 1. Extract the script content containing window.initActivityDetails
	// We look for the script tag that contains "window.initActivityDetails"
	scriptContent := findScriptContent(htmlContent, "window.initActivityDetails")
	if scriptContent == "" {
		return nil, fmt.Errorf("script containing window.initActivityDetails not found")
	}

	// 2. Extract the object passed to initActivityDetails
	// Format: window.initActivityDetails({...}, "activity-details-container");
	// We want the {...} part.
	// Since it's JS, we can use a simple regex to find the function call arguments,
	// but using a JS parser is safer. However, we have goja.
	// Let's wrap it in a function and return the object.
	vm := goja.New()
	
	// Mock window object
	_, err := vm.RunString(`
		var window = {
			initActivityDetails: function(data, container) {
				this.data = data;
			},
			addEventListener: function() {},
			document: {
				addEventListener: function() {}
			}
		};
		var document = window.document;
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to setup JS VM: %w", err)
	}

	// Run the extracted script
	_, err = vm.RunString(scriptContent)
	if err != nil {
		return nil, fmt.Errorf("failed to run script: %w", err)
	}

	// Get the data
	val := vm.Get("window").ToObject(vm).Get("data")
	if val == nil {
		return nil, fmt.Errorf("failed to get data from window object")
	}

	// Export to Go map/interface
	var rawData map[string]interface{}
	err = vm.ExportTo(val, &rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to export JS object: %w", err)
	}

	// 3. Map to our struct
	details := &ActivityDetails{}

	// Contact
	if contactMap, ok := rawData["contact"].(map[string]interface{}); ok {
		details.Contact.ID = int(getInt(contactMap["id"]))
		details.Contact.FirstName, _ = contactMap["firstName"].(string)
		details.Contact.Name, _ = contactMap["name"].(string)
		details.Contact.Image, _ = contactMap["image"].(string)
	}

	// Activity
	if activityMap, ok := rawData["activity"].(map[string]interface{}); ok {
		details.Activity.ID = int(getInt(activityMap["id"]))
		details.Activity.Title, _ = activityMap["title"].(string)
		details.Activity.DateString, _ = activityMap["dateString"].(string)
		details.Activity.StartDateTime, _ = activityMap["startDateTime"].(string)
		details.Activity.MeetingTime, _ = activityMap["meetingTime"].(string)
		details.Activity.Description, _ = activityMap["description"].(string)
	}

	// Attendance Types Order
	attendanceOrder := make(map[string]int)
	if typesMap, ok := rawData["attendanceTypes"].(map[string]interface{}); ok {
		for _, v := range typesMap {
			if typesList, ok := v.([]interface{}); ok {
				for i, t := range typesList {
					if typeMap, ok := t.(map[string]interface{}); ok {
						idStr := getString(typeMap["id"])
						if idStr == "" {
							idStr = strconv.FormatInt(getInt(typeMap["id"]), 10)
						}
						attendanceOrder[idStr] = i
					}
				}
			}
		}
	}

	// Attendances
	details.Attendances = []Attendance{}
	
	attendancesMap, _ := rawData["attendances"].(map[string]interface{})
	attendanceContactsMap, _ := rawData["attendanceContacts"].(map[string]interface{})

	for id, attData := range attendancesMap {
		att, ok := attData.(map[string]interface{})
		if !ok {
			continue
		}

		typeId := getString(att["attendanceTypeId"])
		order, ok := attendanceOrder[typeId]
		if !ok {
			order = 999 // fallback for unknown types
		}

		attendance := Attendance{
			AttendanceTypeName: getString(att["attendanceTypeName"]),
			Comment:            getString(att["comment"]),
			Order:              order,
		}

		// Merge contact info
		if contactData, ok := attendanceContactsMap[id].(map[string]interface{}); ok {
			attendance.FirstName = getString(contactData["firstName"])
			attendance.Name = getString(contactData["name"])
		}

		details.Attendances = append(details.Attendances, attendance)
	}

	// Sort Attendances
	sort.Slice(details.Attendances, func(i, j int) bool {
		if details.Attendances[i].Order != details.Attendances[j].Order {
			return details.Attendances[i].Order < details.Attendances[j].Order
		}
		return details.Attendances[i].Name < details.Attendances[j].Name
	})

	return details, nil
}

func findScriptContent(htmlContent, search string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var content string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			// Get the text content of the script
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				text := n.FirstChild.Data
				if strings.Contains(text, search) {
					content = text
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
			if content != "" {
				return
			}
		}
	}
	f(doc)
	return content
}

func getInt(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	}
	return 0
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type SubscriptionForm struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	SubscriberCount int    `json:"subscriber_count"`
}

func parseSubscriptionForms(htmlContent string) ([]SubscriptionForm, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var forms []SubscriptionForm
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, attr := range n.Attr {
				if attr.Key == "data-form-id" {
					id, _ := strconv.Atoi(attr.Val)
					form := SubscriptionForm{ID: id}

					// Find title
					if title := findTitle(n); title != "" {
						form.Title = title
					}

					// Find count
					form.SubscriberCount = findSubscriberCount(n)

					forms = append(forms, form)
					// Don't traverse deeper into this form's div looking for other forms
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return forms, nil
}

func findTitle(n *html.Node) string {
	// Look for h4 -> a
	// The snippet shows <h4><a>...</a></h4>
	var search func(*html.Node) string
	search = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "h4" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "a" {
					return getText(c)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if res := search(c); res != "" {
				return res
			}
		}
		return ""
	}
	return search(n)
}

func findSubscriberCount(n *html.Node) int {
	// Look for <a> containing <i class="...fa-users...">
	var search func(*html.Node) int
	search = func(n *html.Node) int {
		// Handle recursion
		// Check current node first if it's an <a>
		if n.Type == html.ElementNode && n.Data == "a" {
			hasIcon := false
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "i" {
					for _, attr := range c.Attr {
						if attr.Key == "class" && strings.Contains(attr.Val, "fa-users") {
							hasIcon = true
							break
						}
					}
				}
			}

			if hasIcon {
				text := getText(n)
				// regex to find the first number
				re := regexp.MustCompile(`(\d+)`)
				matches := re.FindStringSubmatch(text)
				if len(matches) > 1 {
					count, _ := strconv.Atoi(matches[1])
					return count
				}
				// Found icon but no number, return 0 (or -1 to keep searching?)
				// Assuming if icon found, this is the spot
				return 0
			}
		}

		// Recurse children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if res := search(c); res != -1 {
				return res
			}
		}
		return -1
	}

	res := search(n)
	if res == -1 {
		return 0
	}
	return res
}

func getText(n *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		// Skip style and script tags content
		if n.Type == html.ElementNode && (n.Data == "style" || n.Data == "script") {
			return
		}
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.TrimSpace(sb.String())
}

// parseEntryIDs parses the HTML content and returns a list of entry IDs.
// It looks for elements with id="entry-{ID}".
func parseEntryIDs(htmlContent string) ([]int, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var ids []int
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "id" && strings.HasPrefix(attr.Val, "entry-") {
					// Check if it's exactly "entry-{ID}" and not "entry-parent-..."
					// The prefix check is loose, so let's check the format
					idStr := strings.TrimPrefix(attr.Val, "entry-")
					// Ensure the rest is just digits
					if id, err := strconv.Atoi(idStr); err == nil {
						ids = append(ids, id)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return ids, nil
}

// stripHTML removes all HTML tags and returns the plain text content.
func stripHTML(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	text := getText(doc)

	// Split by newlines
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

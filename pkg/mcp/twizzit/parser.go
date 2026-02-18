package twizzit

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

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

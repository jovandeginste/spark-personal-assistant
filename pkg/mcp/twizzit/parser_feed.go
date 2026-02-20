package twizzit

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type FeedEvent struct {
	ActivityID int    `json:"ActivityID"`
	Time       string `json:"Time"`
	Title      string `json:"Title"`
	URL        string `json:"URL"`
}

func parseEventsFeed(htmlContent string) ([]FeedEvent, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var events []FeedEvent
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			var class string
			var idStr string
			var dateStr string

			for _, a := range n.Attr {
				if a.Key == "class" {
					class = a.Val
				}
				if a.Key == "data-id" {
					idStr = a.Val
				}
				if a.Key == "data-date" {
					dateStr = a.Val
				}
			}

			// check if this div is an activity row
			if strings.Contains(class, "activity") && idStr != "" && dateStr != "" {
				id, err := strconv.Atoi(idStr)
				if err == nil {
					event := FeedEvent{
						ActivityID: id,
						Time:       dateStr,
						URL:        fmt.Sprintf("https://app.twizzit.com/v2/feed/activity/%d", id),
					}

					// Now extract the title.
					title := extractTitle(n)
					event.Title = title
					events = append(events, event)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return events, nil
}

func extractTitle(n *html.Node) string {
	var title string

	// Helper function to check if a node has a specific class
	hasClass := func(node *html.Node, className string) bool {
		for _, a := range node.Attr {
			if a.Key == "class" && strings.Contains(a.Val, className) {
				return true
			}
		}
		return false
	}

	var findStrong func(*html.Node) bool
	findStrong = func(node *html.Node) bool {
		if node.Type == html.ElementNode && node.Data == "strong" {
			// Ensure it's not the day number inside activity-date
			parent := node.Parent
			inDate := false
			for p := parent; p != nil && p != n; p = p.Parent {
				if hasClass(p, "activity-date") {
					inDate = true
					break
				}
			}

			if !inDate && node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
				title = strings.TrimSpace(node.FirstChild.Data)
				// Handle escaped entities
				title = html.UnescapeString(title)
				return true
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if findStrong(c) {
				return true
			}
		}
		return false
	}

	findStrong(n)
	return title
}

package twizzit

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type ContactSearchResult struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	FirstName string `json:"first_name"`
	Gender    string `json:"gender"`
	DOB       string `json:"dob"`
	Address   string `json:"address"`
	Number    string `json:"number"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
}

func parseContactSearchResults(htmlContent string) ([]ContactSearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []ContactSearchResult
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "dl-row") {
					contact := parseContactRow(n)
					results = append(results, contact)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return results, nil
}

func parseContactRow(n *html.Node) ContactSearchResult {
	var contact ContactSearchResult
	var cells []string

	// Find all dl-cell-content divs
	var findCells func(*html.Node)
	findCells = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "dl-cell-content") {
					cells = append(cells, strings.TrimSpace(getText(n)))
					// Don't recurse into this cell's children for other cells
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findCells(c)
		}
	}
	findCells(n)

	// The columns are fixed based on the request:
	// 0: ID
	// 1: Name
	// 2: First Name
	// 3: Gender
	// 4: DOB
	// 5: Address
	// 6: Shirt Number
	// 7: Email
	// 8: Mobile
	// Note: The HTML structure provided shows an image column as well, let's check the example.

	// In the example HTML:
	// Cell 1: 3889524 (ID)
	// Cell 2: Vandeginste (Name)
	// Cell 3: Jo (First Name)
	// Cell 4: Man (Gender)
	// Cell 5: 25/08/1983 (DOB)
	// Cell 6: Stationsstraat 141 / b (Address)
	// Cell 7: 13 (Number)
	// Cell 8: jo.vandeginste@gmail.com (Email)
	// Cell 9: +32 496727382 (Mobile)

	// The provided example HTML structure actually has cells nested in columns.
	// But our recursive findCells should flatten them in order of appearance.

	if len(cells) >= 9 {
		// Parse ID
		// The first cell is the ID
		// Actually, let's look at the example again.
		// 1. checkbox (not dl-cell-content)
		// 2. ID (dl-cell-content) -> Cell 0
		// 3. Image (profile-image, NOT dl-cell-content usually?)
		// Wait, looking at the HTML example:
		// <div class="col py-2 d-flex dl-cell"> <div class="dl-cell-content" title="3889524"> 3889524 </div> </div>
		// <div class="col ..."> <div class="profile-image ..."> ... </div> </div>  <- NO dl-cell-content here?
		// Actually, the example shows:
		// <div class="col py-2 d-flex dl-cell"> <div class="dl-cell-content" title="Vandeginste"> ...
		// So the image column might NOT have a dl-cell-content class, or at least our findCells logic relies on that class.

		// If the image column doesn't have `dl-cell-content`, it will be skipped by `findCells`.
		// Let's assume `findCells` captures the text fields.

		// Index mapping based on "dl-cell-content" occurrence in example:
		// 0: ID
		// 1: Name
		// 2: First Name
		// 3: Gender (Man/Vrouw)
		// 4: DOB
		// 5: Address
		// 6: Number
		// 7: Email
		// 8: Mobile

		var id int
		fmt.Sscanf(cells[0], "%d", &id)
		contact.ID = id
		contact.Name = cells[1]
		contact.FirstName = cells[2]
		contact.Gender = cells[3]
		contact.DOB = cells[4]
		contact.Address = cells[5]
		contact.Number = cells[6]
		contact.Email = cells[7]
		contact.Mobile = cells[8]
	}

	return contact
}

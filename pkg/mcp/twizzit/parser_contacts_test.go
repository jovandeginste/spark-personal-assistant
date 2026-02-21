package twizzit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseContactSearchResults(t *testing.T) {
	htmlContent := `
<li class="dl-row">
  <div class="row" onclick="contactRelation(3889524);">
    <div class="col-1 d-flex align-items-center py-2">
      <input type="checkbox" class="ml-lg-3" onclick="event.stopPropagation(); toggleActionButton();" name="result-check[]" value="3889524">
    </div>
    <div class="col py-2 d-flex dl-cell">
      <div class="dl-cell-content" title="3889524">
        3889524
      </div>
    </div>
    <div class="col py-2 d-flex dl-cell d-lg-flex justify-content-center">
      <div class="profile-image profile-image-sm">
        <div class="image" style="background-image: url('/public/photos/3889524/s/ee7ce1dfc7cb5d56ec1155b1111d88b2b8cc84ff.png')">
        </div>
      </div>
    </div>
    <div class="col py-2 d-flex dl-cell">
      <div class="dl-cell-content" title="Vandeginste">
        Vandeginste
      </div>
    </div>
    <div class="col py-2 d-flex dl-cell">
      <div class="dl-cell-content" title="Jo">
        Jo
      </div>
    </div>
    <div class="col py-2 d-none d-lg-flex dl-cell">
      <div class="dl-cell-content" title="Man">
        Man
      </div>
    </div>
    <div class="col py-2 d-none d-lg-flex dl-cell">
      <div class="dl-cell-content" title="25/08/1983">
        25/08/1983
      </div>
    </div>
    <div class="col py-2 d-none d-lg-flex dl-cell">
      <div class="dl-cell-content" title="Stationsstraat 141 / b">
        Stationsstraat 141 / b
      </div>
    </div>
    <div class="col py-2 d-none d-lg-flex dl-cell">
      <div class="dl-cell-content" title="13">
        13
      </div>
    </div>
    <div class="col py-2 d-none d-lg-flex dl-cell">
      <div class="dl-cell-content" title="jo.vandeginste@gmail.com">
        jo.vandeginste@gmail.com
      </div>
    </div>
    <div class="col py-2 d-none d-lg-flex dl-cell">
      <div class="dl-cell-content">
        +32 496727382
      </div>
    </div>
  </div>
</li>
`

	results, err := parseContactSearchResults(htmlContent)
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	expected := ContactSearchResult{
		ID:        3889524,
		Name:      "Vandeginste",
		FirstName: "Jo",
		Gender:    "Man",
		DOB:       "25/08/1983",
		Address:   "Stationsstraat 141 / b",
		Number:    "13",
		Email:     "jo.vandeginste@gmail.com",
		Mobile:    "+32 496727382",
	}

	assert.Equal(t, expected, results[0])

	// Print JSON for visual verification
	jsonOutput, err := json.MarshalIndent(results, "", "  ")
	assert.NoError(t, err)
	t.Logf("Parsed JSON: %s", string(jsonOutput))
}

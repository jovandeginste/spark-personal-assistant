package twizzit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEventsFeed(t *testing.T) {
	htmlContent := `
<div class="activity position-relative d-flex my-3 c-pointer" data-id="38568841" data-date="2026-02-20 13:00">
  <div class="activity-date mx-2">
    <small class="text-uppercase">feb</small>
    <strong style="line-height: 1rem; font-size: 1.2rem">20</strong>
  </div>
  <div class="activity-body flex-grow-1 basic-container p-0 ml-1" onclick="loadActivityDetails(38568841);">
    <div class="p-3">
      <div class="pl-2 pr-3 d-flex align-items-center" style="border-left: 3px solid #a3e5d2">
        <div class="flex-grow-1">
          <strong>Extra training gegeven door S2C</strong>
          <div class="text-lowercase">vr 13:00</div>
        </div>
      </div>
    </div>
  </div>
</div>

<div class="activity position-relative d-flex my-3 c-pointer" data-id="36732898" data-date="2026-02-21 20:00">
  <div class="activity-date mx-2">
    <small class="text-uppercase">feb</small>
    <strong style="line-height: 1rem; font-size: 1.2rem">21</strong>
  </div>
  <div class="activity-body flex-grow-1 basic-container p-0 ml-1" onclick="loadActivityDetails(36732898);">
    <div style="background-image: url(https://example.com/img.jpeg);">
    </div>

    <div class="p-3">
      <div class="pl-2 pr-3 d-flex align-items-center" style="border-left: 3px solid #a3e5d2">
        <div class="flex-grow-1">
          <strong>Let&#039;s stick 2gether-quiz</strong>
          <div class="text-lowercase">za 20:00</div>
        </div>
      </div>
    </div>
  </div>
</div>

<div class="activity position-relative d-flex my-3 c-pointer" data-id="37083182" data-date="2026-02-22 10:30">
  <div class="activity-date mx-2">
    <small class="text-uppercase">feb</small>
    <strong style="line-height: 1rem; font-size: 1.2rem">22</strong>
  </div>
  <div class="activity-body flex-grow-1 basic-container p-0 ml-1" onclick="loadActivityDetails(37083182);">
    <div class="p-3">
      <div class="pl-2 pr-3 d-flex align-items-center" style="border-left: 3px solid #e8baa3">
        <div class="flex-grow-1">
          <strong>T1 - Training</strong>
          <div class="text-lowercase">zo 10:30</div>
        </div>
      </div>
    </div>
  </div>
</div>
`
	events, err := parseEventsFeed(htmlContent)
	assert.NoError(t, err)
	assert.Len(t, events, 3)

	assert.Equal(t, 38568841, events[0].ActivityID)
	assert.Equal(t, "2026-02-20 13:00", events[0].Time)
	assert.Equal(t, "Extra training gegeven door S2C", events[0].Title)

	assert.Equal(t, 36732898, events[1].ActivityID)
	assert.Equal(t, "2026-02-21 20:00", events[1].Time)
	assert.Equal(t, "Let's stick 2gether-quiz", events[1].Title)

	assert.Equal(t, 37083182, events[2].ActivityID)
	assert.Equal(t, "2026-02-22 10:30", events[2].Time)
	assert.Equal(t, "T1 - Training", events[2].Title)
}

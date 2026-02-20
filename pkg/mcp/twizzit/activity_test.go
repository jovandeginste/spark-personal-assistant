package twizzit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var sampleActivityHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Activity Details</title>
</head>
<body>
    <div id="content">
        <h1>Activity</h1>
        <script>
            // Some other script
            console.log("hello");
        </script>
        <script>
            window.initActivityDetails(
              {
                /* Secure socket connection */
                socketUrl: "wss://ws.twizzit.com/socket/activityFeed",
                view: "info",
                contact: {
                  id: 3889524,
                  firstName: "Jo",
                  name: "Vandeginste",
                  image: "/public/photos/3889524/s/ee7ce1dfc7cb5d56ec1155b1111d88b2b8cc84ff.png",
                },
                activity: {
                  id: 35997689,
                  title: "Rotselaar T-1 - Genk T-1",
                  dateString: "26/02/2026 20:30 - 22:00",
                  startDateTime: "2026-02-26T20:30:00+01:00",
                  meetingTime: "20:15",
                  score: "",
                  eventType: 3,
                  isFavorite: false,
                  isFuture: true,
                  address: null || "",
                  description: "Some description",
                  attendanceOrForm: 1,
                  organization: 32032,
                  createdById: 1,
                  series: "Trimmers VHL - TR F (Trimmers)",
                  seriesId: "187970",
                },
                attendanceTypes: {
                  "32032": [
                    { "id": "46460", "name": "Aanwezig" },
                    { "id": "46465", "name": "Niet beslist" }
                  ]
                },
                attendances: {
                  "2445982": {
                    "attendanceId": null,
                    "attendanceTypeId": "46465",
                    "attendanceTypeName": "Niet beslist",
                    "value": "0",
                  },
                  "8503671": {
                    "attendanceId": "193789211",
                    "attendanceTypeId": "46460",
                    "attendanceTypeName": "Aanwezig",
                    "value": "1",
                    "comment": "Some comment here",
                  },
                },
                attendanceContacts: {
                  "8503671": {
                    "id": "8503671",
                    "name": "Holemans",
                    "firstName": "Joppe",
                    "fullName": "Holemans Joppe",
                  },
                  "2445982": {
                    "id": "2445982",
                    "name": "De Backer",
                    "firstName": "Kurt",
                    "fullName": "De Backer Kurt",
                  }
                }
              },
              "activity-details-container"
            );
        </script>
    </div>
</body>
</html>
`

func TestParseActivityDetails(t *testing.T) {
	details, err := parseActivityDetails(sampleActivityHTML)
	assert.NoError(t, err)
	assert.NotNil(t, details)

	// Check Contact
	assert.Equal(t, 3889524, details.Contact.ID)
	assert.Equal(t, "Jo", details.Contact.FirstName)
	assert.Equal(t, "Vandeginste", details.Contact.Name)

	// Check Activity
	assert.Equal(t, 35997689, details.Activity.ID)
	assert.Equal(t, "Rotselaar T-1 - Genk T-1", details.Activity.Title)
	assert.Equal(t, "2026-02-26T20:30:00+01:00", details.Activity.StartDateTime)

	// Check Attendances
	assert.Len(t, details.Attendances, 2)

	// Check Sort Order (Aanwezig comes before Niet beslist in our mocked attendanceTypes)
	// Joppe is "Aanwezig" (order 0), Kurt is "Niet beslist" (order 1)
	assert.Equal(t, "Joppe", details.Attendances[0].FirstName)
	assert.Equal(t, "Aanwezig", details.Attendances[0].AttendanceTypeName)
	assert.Equal(t, "Some comment here", details.Attendances[0].Comment)

	assert.Equal(t, "Kurt", details.Attendances[1].FirstName)
	assert.Equal(t, "Niet beslist", details.Attendances[1].AttendanceTypeName)
	assert.Equal(t, "", details.Attendances[1].Comment)

    // Optional: Print JSON for verification
    b, _ := json.MarshalIndent(details, "", "  ")
    t.Log(string(b))
}

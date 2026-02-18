package twizzit

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseSubscriptionForms(t *testing.T) {
	// JSON snippet provided in prompt is actually HTML content inside a JSON string.
	// We simulate the extracted HTML content here.
	htmlContent := `
<style>
    #subscription-splash .zerostate-image {
        background-image: url('/v2/./images/splash/form_splash.png');
    }
</style>

<div>
                    
    <div class="form basic-container  mb-3 " data-form-id="276600" data-form-order="276600">
                <div class="row name">
    <div class="col-12 d-flex align-items-center justify-content-between">
        <h4 class="flex-grow-1">
                                            <a href="/v2/form/276600/overview">infosessies scheids feb. 2026</a>
                                        
            <span class="active-icon text-muted" style="font-size:0.825rem;" data-toggle="tooltip" title="Actief formulier ">&nbsp;<i class="text-primary fas fa-circle" ></i></span>
            <span class="inactive-icon text-muted" style="font-size:0.825rem;" data-toggle="tooltip" title="Inactief formulier ">&nbsp;<i class="fal fa-minus-circle"></i></span>
                                            <div class="badge badge-pill badge-warning mr-3">Template</div>
                                    </h4>
                                    <button type="button" class="border-0 p-2 float-right text-secondary no-caret bg-transparent dropdown-toggle" 
                                            data-toggle="dropdown" aria-haspopup="true" aria-expanded="false"><i class="fas fa-ellipsis-v"></i></button>
            <div class="dropdown-menu dropdown-menu-right">
                                                <a role="button" class="dropdown-item tw-pointer" onclick="toggleActiveForm(276600);">
                                                            <span class="activate-button-text" style="display: none;">Activeren</span>
                        <span class="deactivate-button-text">Deactiveren</span>
                                                    </a>
                                                <div class="dropdown-divider"></div>
                <a role="button" class="dropdown-item tw-pointer" onclick="deleteForm(276600);">Verwijderen</a>
            </div>
        
    </div>
</div>
<div class="row">
    <div class="col-12">
                            </div>
</div>
<div class="row mb-3"></div>
<div class="row">
    <div class="col-12 d-flex justify-content-start">
                                                                    <a class="flex-nowrap btn-feedback mr-3" href="/v2/form/276600/details/builder">
                    <i class="fal fa-edit"></i> Formulier aanpassen                                </a>
                                        <a class="btn-feedback flex-nowrap mr-3" href="/v2/form/276600/entries">
                <i class="fa fa-users" aria-hidden="true"></i> 8 Inschrijvingen                            </a>
                                                        </div>
</div>
</div>

</div>

<input type="hidden" name="offset" value="20" />

<script type="text/javascript">
$(function() {
        $('[data-toggle="tooltip"]').tooltip({animation:false});
});
</script>
`

	forms, err := parseSubscriptionForms(htmlContent)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(forms) != 1 {
		t.Fatalf("Expected 1 form, got %d", len(forms))
	}

	form := forms[0]

	if form.ID != 276600 {
		t.Errorf("Expected ID 276600, got %d", form.ID)
	}

	expectedTitle := "infosessies scheids feb. 2026"
	if form.Title != expectedTitle {
		t.Errorf("Expected Title '%s', got '%s'", expectedTitle, form.Title)
	}

	if form.SubscriberCount != 8 {
		t.Errorf("Expected SubscriberCount 8, got %d", form.SubscriberCount)
	}
}

func TestParseEntryIDs(t *testing.T) {
	htmlContent := `
<style>
    @media (max-width: 992px) {
        .dl-body{
            background: transparent;
        }
        .dl-item {
            display: block!important;
            margin:0.5rem;
            border-radius: 0.75rem;
            background-color: white;
        }
        .dl-item .dl-cell-content{
            display:block;
        }

    }
</style>
    <div class="dl-row" id="entry-9794299">
        <div class="row dl-item" onclick="profileManager.openFormEntryV3(276600, 9794299);">
            <div class="col-1 d-flex align-items-center py-2">
                <input type="checkbox" class="ml-lg-3" onclick="event.stopPropagation(); toggleActionButton();" name="result-check[]" value="9794299">
            </div>
            
                                    
                                                                                                                                                                                                </div>
    </div>

                                    <div class="entry-parent d-none" id="entry-parent-9794299-5941973">
                                                                </div>
                <div class="dl-row" id="entry-9792007">
        <div class="row dl-item" onclick="profileManager.openFormEntryV3(276600, 9792007);">
            <div class="col-1 d-flex align-items-center py-2">
                <input type="checkbox" class="ml-lg-3" onclick="event.stopPropagation(); toggleActionButton();" name="result-check[]" value="9792007">
            </div>
            
                                    
                                                                                                                                                                                                </div>
    </div>
`
	ids, err := parseEntryIDs(htmlContent)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	sort.Ints(ids)
	expected := []int{9794299, 9792007}
	sort.Ints(expected)

	if !reflect.DeepEqual(ids, expected) {
		t.Errorf("Expected IDs %v, got %v", expected, ids)
	}
}

func TestStripHTML(t *testing.T) {
	htmlContent := `
<div>
    <h1>Title</h1>
    <style>
        .hidden { display: none; }
    </style>
    <p>Some text</p>
    <script>
        console.log("should be removed");
    </script>
    <br>
    
    <p>More text</p>
</div>
`
	// verify:
	// 1. Title is present
	// 2. Style content is removed
	// 3. Script content is removed
	// 4. Consecutive newlines collapsed (Title\nSome text\nMore text)
	// 5. Leading/trailing whitespace trimmed
	expected := "Title\nSome text\nMore text"

	result := stripHTML(htmlContent)

	if result != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, result)
	}

	if contains(result, "display: none") {
		t.Error("Expected style content to be removed")
	}
	if contains(result, "console.log") {
		t.Error("Expected script content to be removed")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && s[0:len(substr)] == substr) || contains(s[1:], substr))
}

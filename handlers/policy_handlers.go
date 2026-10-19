package handlers

import (
	"net/http"

	"convention-management-system/services"
)

// CommunityGuidelinesHandler displays the public Community Guidelines.
func CommunityGuidelinesHandler(w http.ResponseWriter, r *http.Request) {
	Render(w, "guidelines.html", nil)
}

// PrivacyPolicyHandler displays the public Privacy Policy.
func PrivacyPolicyHandler(w http.ResponseWriter, r *http.Request) {
	Render(w, "privacy.html", nil)
}

// TermsOfUseHandler displays the public Terms of Use with active version metadata.
func TermsOfUseHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"ActiveVersion": services.GetActiveTermsVersion(),
		"Declined":      r.URL.Query().Get("declined") == "1",
		"Prompt":        r.URL.Query().Get("prompt") == "1",
	}
	Render(w, "terms.html", data)
}



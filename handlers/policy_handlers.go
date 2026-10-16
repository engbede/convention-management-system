package handlers

import "net/http"

// CommunityGuidelinesHandler displays the public Community Guidelines.
func CommunityGuidelinesHandler(w http.ResponseWriter, r *http.Request) {
	Render(w, "guidelines.html", nil)
}

// PrivacyPolicyHandler displays the public Privacy Policy.
func PrivacyPolicyHandler(w http.ResponseWriter, r *http.Request) {
	Render(w, "privacy.html", nil)
}

// TermsOfUseHandler displays the public Terms of Use.
func TermsOfUseHandler(w http.ResponseWriter, r *http.Request) {
	Render(w, "terms.html", nil)
}


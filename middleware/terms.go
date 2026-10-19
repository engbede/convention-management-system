package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

// RequireTermsAcceptance ensures an authenticated user has accepted the active terms version.
func RequireTermsAcceptance(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := sessions.Store.Get(r, "youth-community")
		if err != nil || session.IsNew {
			http.Redirect(w, r, "/community/login", http.StatusSeeOther)
			return
		}

		userID, ok := session.Values["user_id"].(int)
		if !ok || userID == 0 {
			http.Redirect(w, r, "/community/login", http.StatusSeeOther)
			return
		}

		accepted, requiredVersion, _, err := services.CheckTermsAcceptance(userID)
		if err != nil || !accepted {
			// Check if API / AJAX request
			isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
				strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
				r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
				(strings.HasPrefix(r.URL.Path, "/messages/") && r.URL.Path != "/messages")

			if isJSON {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":            "terms_acceptance_required",
					"message":          "You must review and accept the latest Platform Terms of Use before continuing.",
					"required_version": requiredVersion,
				})
				return
			}

			// For web page views, redirect to /terms with prompt
			http.Redirect(w, r, "/terms?prompt=1", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}


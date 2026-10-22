package sessions

import (
	"net/http"

	"github.com/gorilla/sessions"
)

var Store = sessions.NewCookieStore(
	[]byte("replace-this-with-a-long-secret-key"),
)

// GetUserID retrieves the authenticated member's ID from session.
func GetUserID(r *http.Request) int {
	session, err := Store.Get(r, "youth-community")
	if err != nil {
		return 0
	}
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		return 0
	}
	return userID
}

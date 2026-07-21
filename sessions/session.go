package sessions

import "github.com/gorilla/sessions"

var Store = sessions.NewCookieStore(
	[]byte("replace-this-with-a-long-secret-key"),
)

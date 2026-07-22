package handlers

import (
	"database/sql"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/repository"
	"convention-management-system/sessions"
)

func CommunityLogin(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method == http.MethodGet {

		Render(
			w,
			"user login.html",
			nil,
		)

		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := repository.GetUserByUsername(username)

	if err == sql.ErrNoRows {

		http.Error(
			w,
			"Invalid username or password",
			http.StatusUnauthorized,
		)

		return
	}

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid username or password",
			http.StatusUnauthorized,
		)

		return
	}

	session, err := sessions.Store.Get(
		r,
		"youth-community",
	)

	if err != nil {

		http.Error(
			w,
			"Unable to create session",
			http.StatusInternalServerError,
		)

		return
	}

	session.Values["user_id"] = user.ID

	err = session.Save(r, w)

	if err != nil {

		http.Error(
			w,
			"Unable to save session",
			http.StatusInternalServerError,
		)

		return
	}

	http.Redirect(
		w,
		r,
		"/community",
		http.StatusSeeOther,
	)
}

package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"convention-management-system/repository"

	"golang.org/x/crypto/bcrypt"
)

func Login(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method == http.MethodGet {

		Render(
			w,
			"login.html",
			nil,
		)

		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	admin, err := repository.GetAdminByUsername(username)

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
		[]byte(admin.Password),
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

	http.SetCookie(
		w,
		&http.Cookie{
			Name:     "admin_session",
			Value:    strconv.Itoa(admin.ID),
			Path:     "/",
			HttpOnly: true,
		},
	)

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

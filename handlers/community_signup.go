package handlers

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func ShowSignup(
	w http.ResponseWriter,
	r *http.Request,
) {
	Render(
		w,
		"signup.html",
		nil,
	)
}

func Signup(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/signup",
			http.StatusSeeOther,
		)
		return
	}

	fullName := strings.TrimSpace(
		r.FormValue("full_name"),
	)

	username := strings.TrimSpace(
		r.FormValue("username"),
	)

	email := strings.TrimSpace(
		r.FormValue("email"),
	)

	phone := strings.TrimSpace(
		r.FormValue("phone"),
	)

	password := r.FormValue("password")

	confirmPassword := r.FormValue(
		"confirm_password",
	)

	if fullName == "" ||
		username == "" ||
		password == "" {

		http.Error(
			w,
			"Please complete all required fields.",
			http.StatusBadRequest,
		)

		return
	}

	if password != confirmPassword {

		http.Error(
			w,
			"Passwords do not match.",
			http.StatusBadRequest,
		)

		return
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)

	if err != nil {

		http.Error(
			w,
			"Unable to create account.",
			http.StatusInternalServerError,
		)

		return
	}

	user := models.User{

		FullName: fullName,

		Username: username,

		Email: email,

		Phone: phone,

		PasswordHash: string(hash),

		Role: "attendee",
	}

	err = repository.CreateUser(
		&user,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	http.Redirect(
		w,
		r,
		"/community/login",
		http.StatusSeeOther,
	)
}

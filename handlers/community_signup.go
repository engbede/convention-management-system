package handlers

import (
	"log"
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
	if r.Method == http.MethodPost {
		Signup(w, r)
		return
	}
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

		log.Printf("[Registration Validation] missing required fields for username=%q full_name=%q", username, fullName)
		http.Error(
			w,
			"Please complete all required fields.",
			http.StatusBadRequest,
		)

		return
	}

	if password != confirmPassword {

		log.Printf("[Registration Validation] passwords do not match for username=%q", username)
		http.Error(
			w,
			"Passwords do not match.",
			http.StatusBadRequest,
		)

		return
	}

	// Pre-validate uniqueness across PostgreSQL and SQLite
	if exists, err := repository.UserExistsByUsername(username); err == nil && exists {
		log.Printf("[Registration Validation] username already taken: %q", username)
		http.Error(
			w,
			"Username is already taken.",
			http.StatusBadRequest,
		)
		return
	}

	if email != "" {
		if exists, err := repository.UserExistsByEmail(email); err == nil && exists {
			log.Printf("[Registration Validation] email already registered: %q", email)
			http.Error(
				w,
				"Email is already registered.",
				http.StatusBadRequest,
			)
			return
		}
	}

	if phone != "" {
		if exists, err := repository.UserExistsByPhone(phone); err == nil && exists {
			log.Printf("[Registration Validation] phone already registered: %q", phone)
			http.Error(
				w,
				"Phone number is already registered.",
				http.StatusBadRequest,
			)
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		log.Printf("[Registration Error] bcrypt hashing failed for username=%q: %v", username, err)
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
		log.Printf("[Registration Failure] operation=CreateUser username=%q email=%q err=%v", username, email, err)
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "users.username") || strings.Contains(errStr, "users_username") || (strings.Contains(errStr, "23505") && strings.Contains(errStr, "username")) {
			http.Error(
				w,
				"Username is already taken.",
				http.StatusBadRequest,
			)
			return
		}
		if strings.Contains(errStr, "users.email") || strings.Contains(errStr, "users_email") || (strings.Contains(errStr, "23505") && strings.Contains(errStr, "email")) {
			http.Error(
				w,
				"Email is already registered.",
				http.StatusBadRequest,
			)
			return
		}
		if strings.Contains(errStr, "users.phone") || strings.Contains(errStr, "users_phone") || (strings.Contains(errStr, "23505") && strings.Contains(errStr, "phone")) {
			http.Error(
				w,
				"Phone number is already registered.",
				http.StatusBadRequest,
			)
			return
		}

		http.Error(
			w,
			"Unable to create account. Please try again later.",
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

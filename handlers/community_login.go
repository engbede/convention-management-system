package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
)

// CommunityLogin handles GET (renders login landing page) and POST (authenticates member)
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

	loginInput := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	rememberMe := r.FormValue("remember_me")

	if loginInput == "" || password == "" {
		w.WriteHeader(http.StatusUnauthorized)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "Please enter both username/email and password.",
			"Username": loginInput,
		})
		return
	}

	var (
		user *models.User
		err  error
	)

	// Support authentication using either username or email
	if strings.Contains(loginInput, "@") {
		user, err = repository.GetUserByEmail(loginInput)
		if err == sql.ErrNoRows {
			user, err = repository.GetUserByUsername(loginInput)
		}
	} else {
		user, err = repository.GetUserByUsername(loginInput)
		if err == sql.ErrNoRows {
			user, err = repository.GetUserByEmail(loginInput)
		}
	}

	if err == sql.ErrNoRows || user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "Invalid username or password.",
			"Username": loginInput,
		})
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "An unexpected error occurred. Please try again later.",
			"Username": loginInput,
		})
		return
	}

	// Verify password hash
	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "Invalid username or password.",
			"Username": loginInput,
		})
		return
	}

	// Verify account status
	if !user.IsActive {
		w.WriteHeader(http.StatusForbidden)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "Your account is inactive or has been suspended. Please contact church leadership or administration.",
			"Username": loginInput,
		})
		return
	}

	session, err := sessions.Store.Get(
		r,
		"youth-community",
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "Unable to initialize session. Please try again.",
			"Username": loginInput,
		})
		return
	}

	session.Values["user_id"] = user.ID

	// Extend session duration if remember-me was selected
	if rememberMe == "on" || rememberMe == "true" || rememberMe == "1" {
		session.Options.MaxAge = 30 * 24 * 60 * 60 // 30 days
	} else {
		session.Options.MaxAge = 24 * 60 * 60 // 1 day
	}

	err = session.Save(r, w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Render(w, "user login.html", map[string]interface{}{
			"Error":    "Unable to save session. Please try again.",
			"Username": loginInput,
		})
		return
	}

	http.Redirect(
		w,
		r,
		"/community",
		http.StatusSeeOther,
	)
}

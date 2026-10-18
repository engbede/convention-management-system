package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"

	"golang.org/x/crypto/bcrypt"
)

type adminAttemptRecord struct {
	count       int
	lastAttempt time.Time
}

var (
	adminAttemptsMutex sync.Mutex
	adminAttempts      = make(map[string]adminAttemptRecord)
)

func getAdminRateLimitKey(r *http.Request, username string) string {
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		ip = strings.TrimSpace(parts[0])
	}
	return strings.ToLower(username) + ":" + ip
}

func checkAdminRateLimit(key string) bool {
	adminAttemptsMutex.Lock()
	defer adminAttemptsMutex.Unlock()

	rec, exists := adminAttempts[key]
	if !exists {
		return true
	}

	// 15-minute lockout window
	if time.Since(rec.lastAttempt) > 15*time.Minute {
		delete(adminAttempts, key)
		return true
	}

	return rec.count < 5
}

func recordFailedAdminAttempt(key string) {
	adminAttemptsMutex.Lock()
	defer adminAttemptsMutex.Unlock()

	rec := adminAttempts[key]
	rec.count++
	rec.lastAttempt = time.Now()
	adminAttempts[key] = rec
}

func resetAdminAttempts(key string) {
	adminAttemptsMutex.Lock()
	defer adminAttemptsMutex.Unlock()
	delete(adminAttempts, key)
}

func AdminLogin(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method == http.MethodGet {
		successMsg := r.URL.Query().Get("success")
		Render(
			w,
			"login.html",
			map[string]any{
				"Success": successMsg,
			},
		)
		return
	}

	usernameInput := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	rateKey := getAdminRateLimitKey(r, usernameInput)
	if !checkAdminRateLimit(rateKey) {
		w.WriteHeader(http.StatusTooManyRequests)
		Render(w, "login.html", map[string]any{
			"Error":    "Too many failed login attempts. Please wait 15 minutes before trying again.",
			"Username": usernameInput,
		})
		return
	}

	if usernameInput == "" || password == "" {
		w.WriteHeader(http.StatusUnauthorized)
		Render(w, "login.html", map[string]any{
			"Error":    "Please enter both username and password.",
			"Username": usernameInput,
		})
		return
	}

	var (
		admin models.Admin
		err   error
	)

	if strings.Contains(usernameInput, "@") {
		admin, err = repository.GetAdminByEmail(usernameInput)
		if err == sql.ErrNoRows {
			admin, err = repository.GetAdminByUsername(usernameInput)
		}
	} else {
		admin, err = repository.GetAdminByUsername(usernameInput)
		if err == sql.ErrNoRows {
			admin, err = repository.GetAdminByEmail(usernameInput)
		}
	}

	if err == sql.ErrNoRows {
		recordFailedAdminAttempt(rateKey)
		w.WriteHeader(http.StatusUnauthorized)
		Render(w, "login.html", map[string]any{
			"Error":    "Invalid username or password",
			"Username": usernameInput,
		})
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Render(w, "login.html", map[string]any{
			"Error":    "A database error occurred. Please try again later.",
			"Username": usernameInput,
		})
		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(admin.Password),
		[]byte(password),
	)

	if err != nil {
		recordFailedAdminAttempt(rateKey)
		w.WriteHeader(http.StatusUnauthorized)
		Render(w, "login.html", map[string]any{
			"Error":    "Invalid username or password",
			"Username": usernameInput,
		})
		return
	}

	// Login successful: reset rate limit attempts
	resetAdminAttempts(rateKey)

	http.SetCookie(
		w,
		&http.Cookie{
			Name:     "admin_session",
			Value:    strconv.Itoa(admin.ID),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
	)

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

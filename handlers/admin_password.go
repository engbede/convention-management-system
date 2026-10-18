package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
)

type AdminForgotPasswordViewData struct {
	ErrorMessage   string
	SuccessMessage string
	Login          string
}

type AdminResetPasswordViewData struct {
	Token          string
	ErrorMessage   string
	SuccessMessage string
	IsValidToken   bool
}

type AdminChangePasswordViewData struct {
	Title          string
	ErrorMessage   string
	SuccessMessage string
}

func AdminForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		Render(w, "admin_forgot_password.html", AdminForgotPasswordViewData{})
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/forgot-password", http.StatusSeeOther)
		return
	}

	loginInput := strings.TrimSpace(r.FormValue("login"))
	if loginInput == "" {
		Render(w, "admin_forgot_password.html", AdminForgotPasswordViewData{
			ErrorMessage: "Please enter your administrator email address or username.",
			Login:        loginInput,
		})
		return
	}

	// Determine base URL
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	var (
		admin models.Admin
		err   error
	)

	if strings.Contains(loginInput, "@") {
		admin, err = repository.GetAdminByEmail(loginInput)
		if err == sql.ErrNoRows {
			admin, err = repository.GetAdminByUsername(loginInput)
		}
	} else {
		admin, err = repository.GetAdminByUsername(loginInput)
		if err == sql.ErrNoRows {
			admin, err = repository.GetAdminByEmail(loginInput)
		}
	}

	if err == nil && admin.ID > 0 {
		recoveryEmail := strings.TrimSpace(admin.Email)
		if recoveryEmail == "" {
			Render(w, "admin_forgot_password.html", AdminForgotPasswordViewData{
				ErrorMessage: "This administrator account has no recovery email address configured. Please contact the Diocesan Technical Lead or database administrator.",
				Login:        loginInput,
			})
			return
		}

		// Rate limiting check: max 5 requests per 15 minutes
		count, rErr := repository.CountRecentResetsByEmail(recoveryEmail, 15*time.Minute)
		if rErr == nil && count >= 5 {
			Render(w, "admin_forgot_password.html", AdminForgotPasswordViewData{
				ErrorMessage: "Too many password recovery requests. Please wait 15 minutes before trying again.",
				Login:        loginInput,
			})
			return
		}

		rawToken, err := services.GenerateSecureToken()
		if err == nil {
			tokenHash := services.HashToken(rawToken)
			expiresAt := time.Now().Add(1 * time.Hour)

			reset := &models.PasswordReset{
				UserID:    admin.ID,
				Email:     recoveryEmail,
				TokenHash: tokenHash,
				ExpiresAt: expiresAt,
				Used:      false,
				CreatedAt: time.Now(),
			}

			if err := repository.CreatePasswordReset(reset); err == nil {
				resetLink := fmt.Sprintf("%s/admin/reset-password?token=%s", strings.TrimRight(baseURL, "/"), rawToken)
				subject := "Diocesan Administrator Password Reset — Methodist Church Nigeria"
				body := fmt.Sprintf("Dear Administrator (%s),\n\nA request to reset your password for the Diocesan Mission Control was received.\n\nClick the link below to set a new password:\n%s\n\nThis link is valid for 1 hour. If you did not make this request, please review your security settings immediately.\n\nMethodist Church Nigeria, Diocese of Apa", admin.Username, resetLink)
				_ = services.SendEmail(recoveryEmail, subject, body)
				log.Printf("[Admin Password Recovery] Reset email queued for admin %s (%s)", admin.Username, recoveryEmail)
			}
		}
	}

	// Always show generic confirmation
	Render(w, "admin_forgot_password.html", AdminForgotPasswordViewData{
		SuccessMessage: "If an administrator account with those credentials exists with a recovery email, instructions have been dispatched. Please check your inbox.",
	})
}

func AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
				ErrorMessage: "Password reset token is missing. Please request a new recovery link.",
				IsValidToken: false,
			})
			return
		}

		tokenHash := services.HashToken(token)
		reset, err := repository.GetValidPasswordResetByTokenHash(tokenHash)
		if err != nil || reset == nil {
			Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
				ErrorMessage: "This password recovery link is invalid or has expired. Please request a new link.",
				IsValidToken: false,
			})
			return
		}

		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if token == "" {
		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			ErrorMessage: "Password reset token is missing.",
			IsValidToken: false,
		})
		return
	}

	tokenHash := services.HashToken(token)
	reset, err := repository.GetValidPasswordResetByTokenHash(tokenHash)
	if err != nil || reset == nil {
		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			ErrorMessage: "This password recovery link is invalid or has expired. Please request a new link.",
			IsValidToken: false,
		})
		return
	}

	if len(password) < 8 {
		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Password must be at least 8 characters long.",
		})
		return
	}

	if password != confirmPassword {
		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Passwords do not match.",
		})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Unable to encrypt password. Please try again.",
		})
		return
	}

	err = repository.UpdateAdminPasswordAndInvalidateReset(reset.UserID, string(newHash), tokenHash)
	if err != nil {
		Render(w, "admin_reset_password.html", AdminResetPasswordViewData{
			ErrorMessage: "This reset link has already been used or expired.",
			IsValidToken: false,
		})
		return
	}

	log.Printf("[Admin Password Reset] Administrator ID %d successfully updated password via reset token", reset.UserID)

	http.Redirect(w, r, "/login?success=Administrator+password+updated+successfully.+Please+sign+in.", http.StatusSeeOther)
}

func AdminChangePassword(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("admin_session")
	if err != nil || cookie == nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	adminID, err := strconv.Atoi(cookie.Value)
	if err != nil || adminID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title: "Change Administrator Password",
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/change-password", http.StatusSeeOther)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	admin, err := repository.GetAdminByID(adminID)
	if err != nil {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title:        "Change Administrator Password",
			ErrorMessage: "Unable to find administrator account. Please log in again.",
		})
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(currentPassword)); err != nil {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title:        "Change Administrator Password",
			ErrorMessage: "Current password does not match our records.",
		})
		return
	}

	if len(newPassword) < 8 {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title:        "Change Administrator Password",
			ErrorMessage: "New password must be at least 8 characters long.",
		})
		return
	}

	if newPassword != confirmPassword {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title:        "Change Administrator Password",
			ErrorMessage: "New passwords do not match.",
		})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title:        "Change Administrator Password",
			ErrorMessage: "Unable to encrypt password. Please try again.",
		})
		return
	}

	if err := repository.UpdateAdminPassword(admin.ID, string(newHash)); err != nil {
		Render(w, "admin_change_password.html", AdminChangePasswordViewData{
			Title:        "Change Administrator Password",
			ErrorMessage: "Failed to update password in database. Please try again later.",
		})
		return
	}

	log.Printf("[Admin Password Change] Admin %s (ID %d) successfully changed password", admin.Username, admin.ID)

	Render(w, "admin_change_password.html", AdminChangePasswordViewData{
		Title:          "Change Administrator Password",
		SuccessMessage: "Your administrator password has been updated successfully.",
	})
}

package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

type ForgotPasswordViewData struct {
	SuccessMessage string
	ErrorMessage   string
	Email          string
}

type ResetPasswordViewData struct {
	Token          string
	ErrorMessage   string
	SuccessMessage string
	IsValidToken   bool
}

func ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		Render(w, "forgot_password.html", ForgotPasswordViewData{})
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		Render(w, "forgot_password.html", ForgotPasswordViewData{
			ErrorMessage: "Please enter your email address.",
			Email:        email,
		})
		return
	}

	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		Render(w, "forgot_password.html", ForgotPasswordViewData{
			ErrorMessage: "Please enter a valid email address.",
			Email:        email,
		})
		return
	}

	// Determine base URL
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	err := services.RequestPasswordReset(baseURL, email)
	if err != nil && strings.Contains(err.Error(), "too many reset requests") {
		Render(w, "forgot_password.html", ForgotPasswordViewData{
			ErrorMessage: "Too many reset requests. Please wait a few minutes before trying again.",
			Email:        email,
		})
		return
	}

	// Always show generic success message (no account enumeration)
	Render(w, "forgot_password.html", ForgotPasswordViewData{
		SuccessMessage: "If an account with that email exists, we have sent a password reset link. Please check your inbox.",
	})
}

func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			Render(w, "reset_password.html", ResetPasswordViewData{
				ErrorMessage: "Password reset token is missing. Please request a new reset link.",
				IsValidToken: false,
			})
			return
		}

		tokenHash := services.HashToken(token)
		reset, err := repository.GetValidPasswordResetByTokenHash(tokenHash)
		if err != nil || reset == nil {
			Render(w, "reset_password.html", ResetPasswordViewData{
				ErrorMessage: "This password reset link is invalid or has expired. Please request a new link.",
				IsValidToken: false,
			})
			return
		}

		Render(w, "reset_password.html", ResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if token == "" {
		Render(w, "reset_password.html", ResetPasswordViewData{
			ErrorMessage: "Password reset token is missing.",
			IsValidToken: false,
		})
		return
	}

	tokenHash := services.HashToken(token)
	reset, err := repository.GetValidPasswordResetByTokenHash(tokenHash)
	if err != nil || reset == nil {
		Render(w, "reset_password.html", ResetPasswordViewData{
			ErrorMessage: "This password reset link is invalid or has expired. Please request a new link.",
			IsValidToken: false,
		})
		return
	}

	if len(password) < 6 {
		Render(w, "reset_password.html", ResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Password must be at least 6 characters long.",
		})
		return
	}

	if password != confirmPassword {
		Render(w, "reset_password.html", ResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Passwords do not match.",
		})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[Password Reset Error] bcrypt hash failed: %v", err)
		Render(w, "reset_password.html", ResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Unable to update password. Please try again.",
		})
		return
	}

	// Update user password and invalidate reset token in atomic transaction
	err = repository.UpdateUserPasswordAndInvalidateReset(reset.UserID, string(newHash), tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			Render(w, "reset_password.html", ResetPasswordViewData{
				ErrorMessage: "This reset link has already been used or expired.",
				IsValidToken: false,
			})
			return
		}
		log.Printf("[Password Reset Error] atomic update failed for user %d: %v", reset.UserID, err)
		Render(w, "reset_password.html", ResetPasswordViewData{
			Token:        token,
			IsValidToken: true,
			ErrorMessage: "Unable to update password. Please try again later.",
		})
		return
	}

	// Invalidate any session for the current user if present
	if session, err := sessions.Store.Get(r, "youth-community"); err == nil {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}

	log.Printf("[Password Reset Success] Password reset successfully for user ID: %d", reset.UserID)

	Render(w, "reset_password.html", ResetPasswordViewData{
		SuccessMessage: "Your password has been reset successfully! You can now log in with your new password.",
		IsValidToken:   false,
	})
}

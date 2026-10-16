package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"
)

// EmailSenderFunc defines the function type for sending emails (can be overridden in tests)
var EmailSenderFunc func(to, subject, body string) error = defaultSendEmail

func defaultSendEmail(to, subject, body string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := os.Getenv("SMTP_FROM")

	if smtpHost == "" || smtpPort == "" || smtpFrom == "" {
		log.Printf("[Email Service] SMTP configuration missing (SMTP_HOST, SMTP_PORT, or SMTP_FROM). Email to %s with subject %q was logged but not sent.", to, subject)
		return nil
	}

	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", smtpFrom, to, subject, body))

	return smtp.SendMail(addr, auth, smtpFrom, []string{to}, msg)
}

// GenerateSecureToken generates a cryptographically secure random token (64 hex characters)
func GenerateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashToken returns the SHA-256 hash of a token string
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// RequestPasswordReset handles token generation, persistence, and sending reset email
func RequestPasswordReset(baseURL, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// 1. Rate limiting check (max 5 requests per 15 minutes)
	count, err := repository.CountRecentResetsByEmail(email, 15*time.Minute)
	if err == nil && count >= 5 {
		log.Printf("[Password Reset Rate Limit] Too many reset requests for email: %s", email)
		return fmt.Errorf("too many reset requests, please wait before trying again")
	}

	// 2. Look up user by email
	user, err := repository.GetUserByEmail(email)
	if err != nil || user == nil {
		// Return nil so we don't disclose whether the email exists
		log.Printf("[Password Reset] Reset requested for non-existent email: %s", email)
		return nil
	}

	// 3. Generate secure token
	rawToken, err := GenerateSecureToken()
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	tokenHash := HashToken(rawToken)
	expiresAt := time.Now().Add(1 * time.Hour)

	// 4. Save token hash in database
	reset := &models.PasswordReset{
		UserID:    user.ID,
		Email:     user.Email,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	if err := repository.CreatePasswordReset(reset); err != nil {
		return fmt.Errorf("failed to save password reset record: %w", err)
	}

	// 5. Send reset email
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", baseURL, rawToken)

	subject := "Reset Your Youth Community Password"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 8px;">
			<h2 style="color: #198754;">Password Reset Request</h2>
			<p>Hello %s,</p>
			<p>We received a request to reset your password for your Apa Diocesan Youth Fellowship account.</p>
			<p>Click the button below to reset your password. This link is valid for <strong>1 hour</strong> and can only be used once.</p>
			<div style="margin: 30px 0;">
				<a href="%s" style="background-color: #198754; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 50px; font-weight: bold; display: inline-block;">Reset Password</a>
			</div>
			<p style="color: #666; font-size: 0.9rem;">Or copy and paste this link into your browser:<br><a href="%s">%s</a></p>
			<hr style="border: 0; border-top: 1px solid #eee; margin: 20px 0;">
			<p style="color: #999; font-size: 0.8rem;">If you did not request this password reset, please ignore this email. Your account remains secure.</p>
		</div>
	`, user.FullName, resetLink, resetLink, resetLink)

	if err := EmailSenderFunc(user.Email, subject, body); err != nil {
		log.Printf("[Email Service Error] Failed to send password reset email to %s: %v", user.Email, err)
		return nil
	}

	log.Printf("[Password Reset] Generated reset link for user %s (%s)", user.Username, user.Email)
	return nil
}

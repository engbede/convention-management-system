package models

import "time"

// TermsAcceptance represents an immutable audit log record of a user accepting platform terms.
type TermsAcceptance struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	TermsVersion string    `json:"terms_version"`
	AcceptedAt   time.Time `json:"accepted_at"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
}

// TermsStatusResponse provides the client with terms compliance information.
type TermsStatusResponse struct {
	Accepted        bool             `json:"accepted"`
	RequiredVersion string           `json:"required_version"`
	UserVersion     string           `json:"user_version,omitempty"`
	AcceptedAt      *time.Time       `json:"accepted_at,omitempty"`
	Record          *TermsAcceptance `json:"record,omitempty"`
}

// TermsAcceptRequest payload submitted by user upon agreement.
type TermsAcceptRequest struct {
	Version string `json:"version"`
}


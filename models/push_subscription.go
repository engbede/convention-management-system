package models

import "time"

// PushSubscription represents a registered browser Web Push subscription.
type PushSubscription struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PushSubscriptionPayload is the incoming payload from the browser PushManager.subscribe().
type PushSubscriptionPayload struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	UserAgent string `json:"user_agent,omitempty"`
}

// CallNotificationPayload is the JSON data dispatched in Web Push events for incoming calls and cancellations.
type CallNotificationPayload struct {
	Type           string `json:"type"` // "call_offer", "call_cancel"
	CallID         int    `json:"call_id"`
	ConversationID int    `json:"conversation_id"`
	CallerID       int    `json:"caller_id"`
	CallerName     string `json:"caller_name"`
	CallerAvatar   string `json:"caller_avatar,omitempty"`
	CallType       string `json:"call_type"` // "audio" or "video"
	Title          string `json:"title"`
	Body           string `json:"body"`
	URL            string `json:"url"`
	Tag            string `json:"tag"`
	ExpiresAt      int64  `json:"expires_at"` // Unix epoch timestamp
}


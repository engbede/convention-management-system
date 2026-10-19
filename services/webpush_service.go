package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"convention-management-system/models"
	"convention-management-system/repository"

	"github.com/SherClockHolmes/webpush-go"
)

var (
	vapidMu         sync.RWMutex
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubject    string
)

func init() {
	initVAPIDKeys()
}

func initVAPIDKeys() {
	vapidMu.Lock()
	defer vapidMu.Unlock()

	pub := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	priv := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
	sub := strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))

	if sub == "" {
		sub = "mailto:youth@dioceseofapa.org"
	}
	vapidSubject = sub

	if pub != "" && priv != "" {
		vapidPublicKey = pub
		vapidPrivateKey = priv
		log.Printf("[WebPush] Initialized VAPID configuration with configured environment keys.")
		return
	}

	// Generate fallback development VAPID keys if not present
	genPriv, genPub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Printf("[WebPush] Warning: failed generating fallback VAPID keys: %v", err)
		return
	}

	vapidPublicKey = genPub
	vapidPrivateKey = genPriv
	log.Printf("[WebPush] Generated active fallback VAPID keys for Web Push. Public Key: %s", genPub)
}

// GetVAPIDPublicKey returns the active base64-encoded VAPID public key for browser PushManager subscription.
func GetVAPIDPublicKey() string {
	vapidMu.RLock()
	defer vapidMu.RUnlock()
	return vapidPublicKey
}

// SendCallPushNotification dispatches a Web Push notification to all registered devices of the target user.
func SendCallPushNotification(targetUserID int, payload *models.CallNotificationPayload) error {
	vapidMu.RLock()
	pubKey := vapidPublicKey
	privKey := vapidPrivateKey
	sub := vapidSubject
	vapidMu.RUnlock()

	if pubKey == "" || privKey == "" {
		return fmt.Errorf("webpush VAPID keys not configured")
	}

	subscriptions, err := repository.GetPushSubscriptionsByUserID(targetUserID)
	if err != nil {
		return fmt.Errorf("failed fetching push subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		return nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed marshaling call push payload: %w", err)
	}

	// Send to all subscriptions in background goroutines
	for _, s := range subscriptions {
		go func(subRecord models.PushSubscription) {
			sObj := &webpush.Subscription{
				Endpoint: subRecord.Endpoint,
				Keys: webpush.Keys{
					P256dh: subRecord.P256dh,
					Auth:   subRecord.Auth,
				},
			}

			// Calls are urgent and expire in 45s (TTL 45)
			resp, sErr := webpush.SendNotification(payloadBytes, sObj, &webpush.Options{
				Subscriber:      sub,
				VAPIDPublicKey:  pubKey,
				VAPIDPrivateKey: privKey,
				TTL:             45,
				Urgency:         webpush.UrgencyHigh,
				Topic:           fmt.Sprintf("call-%d", payload.CallID),
			})

			if sErr != nil {
				log.Printf("[WebPush] Delivery error for user %d (endpoint: %s...): %v", targetUserID, subRecord.Endpoint[:min(30, len(subRecord.Endpoint))], sErr)
			}

			if resp != nil {
				defer resp.Body.Close()
				// HTTP 404 Not Found or 410 Gone indicates expired/unregistered subscription
				if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
					log.Printf("[WebPush] Subscription expired or unregistered (status %d). Pruning endpoint %s...", resp.StatusCode, subRecord.Endpoint[:min(30, len(subRecord.Endpoint))])
					_ = repository.DeletePushSubscriptionByEndpoint(subRecord.Endpoint)
				}
			}
		}(s)
	}

	return nil
}

// SendCallCancelPush sends a cancellation push event to dismiss ringing notifications.
func SendCallCancelPush(targetUserID int, callID int, convID int) error {
	cancelPayload := &models.CallNotificationPayload{
		Type:           "call_cancel",
		CallID:         callID,
		ConversationID: convID,
		Tag:            fmt.Sprintf("call-%d", callID),
	}
	return SendCallPushNotification(targetUserID, cancelPayload)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


package services

import (
	"sync"
	"time"
)

// StoredCallOffer represents an active incoming call offer cached in memory
// so that callees arriving via background push or cross-page redirects
// can retrieve the authoritative SDP offer and establish the connection.
type StoredCallOffer struct {
	CallID         int       `json:"call_id"`
	ConversationID int       `json:"conversation_id"`
	CallerID       int       `json:"caller_id"`
	ReceiverID     int       `json:"receiver_id"`
	CallerName     string    `json:"caller_name"`
	CallerAvatar   string    `json:"caller_avatar"`
	CallType       string    `json:"call_type"`
	SDP            string    `json:"sdp"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

var (
	callOffersMu sync.RWMutex
	callOffers   = make(map[int]*StoredCallOffer)
)

// StoreCallOffer saves or updates an active call offer in memory.
func StoreCallOffer(offer *StoredCallOffer) {
	if offer == nil || offer.CallID <= 0 {
		return
	}
	callOffersMu.Lock()
	defer callOffersMu.Unlock()
	callOffers[offer.CallID] = offer
}

// GetStoredCallOffer retrieves an active call offer by CallID.
func GetStoredCallOffer(callID int) (*StoredCallOffer, bool) {
	callOffersMu.RLock()
	defer callOffersMu.RUnlock()
	offer, ok := callOffers[callID]
	if !ok || offer == nil {
		return nil, false
	}
	if time.Now().After(offer.ExpiresAt) {
		return nil, false
	}
	return offer, true
}

// RemoveStoredCallOffer deletes an active call offer once answered, declined, or cancelled.
func RemoveStoredCallOffer(callID int) {
	if callID <= 0 {
		return
	}
	callOffersMu.Lock()
	defer callOffersMu.Unlock()
	delete(callOffers, callID)
}


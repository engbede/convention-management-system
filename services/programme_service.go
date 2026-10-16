package services

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"
)

var (
	youtubeIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	// In-memory rate limiting map as a fast layer: key = "progID:userID", val = time.Time
	chatRateLimitMu sync.Mutex
	lastChatTimes   = make(map[string]time.Time)
)

// ExtractYouTubeVideoID extracts an 11-character YouTube video ID from a URL or raw ID string.
func ExtractYouTubeVideoID(input string) (string, error) {
	clean := strings.TrimSpace(input)
	if clean == "" {
		return "", errors.New("YouTube link or ID cannot be empty")
	}

	// 1. Direct 11-char ID check
	if youtubeIDRegex.MatchString(clean) {
		return clean, nil
	}

	// 2. Parse URL
	parsed, err := url.Parse(clean)
	if err != nil {
		return "", errors.New("invalid URL format")
	}

	host := strings.ToLower(parsed.Host)
	path := parsed.Path

	// Handle short URL: youtu.be/VIDEO_ID
	if strings.Contains(host, "youtu.be") {
		trimmedPath := strings.TrimPrefix(path, "/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) > 0 && youtubeIDRegex.MatchString(parts[0]) {
			return parts[0], nil
		}
	}

	// Handle youtube.com / m.youtube.com / www.youtube.com
	if strings.Contains(host, "youtube.com") {
		// e.g. /watch?v=VIDEO_ID
		if strings.HasPrefix(path, "/watch") {
			v := parsed.Query().Get("v")
			if youtubeIDRegex.MatchString(v) {
				return v, nil
			}
		}
		// e.g. /live/VIDEO_ID or /embed/VIDEO_ID or /v/VIDEO_ID
		prefixes := []string{"/live/", "/embed/", "/v/"}
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				id := strings.TrimPrefix(path, prefix)
				parts := strings.Split(id, "/")
				if len(parts) > 0 && youtubeIDRegex.MatchString(parts[0]) {
					return parts[0], nil
				}
			}
		}
	}

	return "", errors.New("unable to extract a valid 11-character YouTube video ID")
}

// CanUserViewProgramme verifies if a user has access to view a programme.
func CanUserViewProgramme(p *models.Programme, currentUserID int, isAdmin bool) bool {
	if p == nil {
		return false
	}
	if p.Visibility == models.ProgrammeVisibilityPublic {
		return true
	}
	// members_only requires login or admin
	return currentUserID > 0 || isAdmin
}

// CanUserManageProgramme checks whether the user is the organizer or an admin.
func CanUserManageProgramme(p *models.Programme, currentUserID int, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	if currentUserID > 0 && p != nil && p.OrganizerID == currentUserID {
		return true
	}
	return false
}

// CanUserModerateProgramme checks whether the user is an admin, organizer, or assigned moderator.
func CanUserModerateProgramme(p *models.Programme, currentUserID int, isAdmin bool) bool {
	if CanUserManageProgramme(p, currentUserID, isAdmin) {
		return true
	}
	if currentUserID <= 0 || p == nil {
		return false
	}
	isMod, err := repository.IsProgrammeModerator(p.ID, currentUserID)
	return err == nil && isMod
}

// TransitionProgrammeStatus validates and executes a lifecycle transition.
func TransitionProgrammeStatus(programmeID int, targetStatus string, reason string, currentUserID int, isAdmin bool) error {
	p, err := repository.GetProgrammeByID(programmeID)
	if err != nil {
		return errors.New("programme not found")
	}

	if !CanUserModerateProgramme(p, currentUserID, isAdmin) {
		return errors.New("permission denied: only the organizer, assigned moderators, or admins can update programme status")
	}

	// Validate allowed lifecycle transitions
	currentStatus := p.Status
	switch currentStatus {
	case models.ProgrammeStatusDraft:
		if targetStatus != models.ProgrammeStatusScheduled &&
			targetStatus != models.ProgrammeStatusLive &&
			targetStatus != models.ProgrammeStatusCancelled {
			return fmt.Errorf("invalid transition from '%s' to '%s'", currentStatus, targetStatus)
		}
	case models.ProgrammeStatusScheduled:
		if targetStatus != models.ProgrammeStatusLive &&
			targetStatus != models.ProgrammeStatusCancelled {
			return fmt.Errorf("invalid transition from '%s' to '%s'", currentStatus, targetStatus)
		}
	case models.ProgrammeStatusLive:
		if targetStatus != models.ProgrammeStatusEnded &&
			targetStatus != models.ProgrammeStatusCancelled {
			return fmt.Errorf("invalid transition from '%s' to '%s'", currentStatus, targetStatus)
		}
	case models.ProgrammeStatusEnded:
		return errors.New("cannot change status of an ended programme")
	case models.ProgrammeStatusCancelled:
		return errors.New("cannot change status of a cancelled programme")
	default:
		return fmt.Errorf("unknown current status '%s'", currentStatus)
	}

	if targetStatus == models.ProgrammeStatusCancelled && strings.TrimSpace(reason) == "" {
		reason = "Cancelled by organizer"
	}

	if err := repository.UpdateProgrammeStatus(programmeID, targetStatus, reason); err != nil {
		return err
	}

	// If going live, trigger 'went_live' notification
	if targetStatus == models.ProgrammeStatusLive {
		_ = TriggerProgrammeWentLiveNotification(p)
	}

	return nil
}

// TriggerProgrammeWentLiveNotification sends broadcast notifications when a stream goes live.
func TriggerProgrammeWentLiveNotification(p *models.Programme) error {
	const notifType = "went_live"
	sent, err := repository.HasProgrammeNotificationBeenSent(p.ID, notifType)
	if err != nil || sent {
		return nil
	}

	// Log that notification has been sent to deduplicate
	if err := repository.LogProgrammeNotificationSent(p.ID, notifType); err != nil {
		return err
	}

	// Notify organizer or community members if available
	msg := fmt.Sprintf("'%s' is now LIVE! Join the online broadcast.", p.Title)
	notif := models.Notification{
		SenderID:   p.OrganizerID,
		ReceiverID: p.OrganizerID, // Self notification & record
		Type:       "stream_live",
		Message:    msg,
	}
	_ = CreateNotification(notif)
	return nil
}

// CheckChatRateLimit verifies whether a user is rate-limited (minimum 2 seconds between chats).
func CheckChatRateLimit(programmeID, userID int) (bool, string) {
	key := fmt.Sprintf("%d:%d", programmeID, userID)

	chatRateLimitMu.Lock()
	lastTime, exists := lastChatTimes[key]
	chatRateLimitMu.Unlock()

	now := time.Now()
	if exists && now.Sub(lastTime) < 2*time.Second {
		return false, "Slow down! Please wait 2 seconds between chat messages."
	}

	// Also check DB timestamp
	dbTime, err := repository.GetLastChatMessageTimeByUser(programmeID, userID)
	if err == nil && !dbTime.IsZero() && now.Sub(dbTime) < 2*time.Second {
		return false, "Slow down! Please wait 2 seconds between chat messages."
	}

	return true, ""
}

// RecordChatMessageTime updates the rate-limit timestamp for a user in a programme.
func RecordChatMessageTime(programmeID, userID int) {
	key := fmt.Sprintf("%d:%d", programmeID, userID)
	chatRateLimitMu.Lock()
	lastChatTimes[key] = time.Now()
	chatRateLimitMu.Unlock()
}

// SendProgrammeChatMessage validates, rate-limits, and persists a live chat message.
func SendProgrammeChatMessage(programmeID, userID int, messageText string) (*models.ProgrammeChatMessage, error) {
	cleanMsg := strings.TrimSpace(messageText)
	if len(cleanMsg) == 0 {
		return nil, errors.New("chat message cannot be empty")
	}
	if len(cleanMsg) > 500 {
		return nil, errors.New("chat message exceeds maximum length of 500 characters")
	}

	p, err := repository.GetProgrammeByID(programmeID)
	if err != nil {
		return nil, errors.New("programme not found")
	}

	if p.IsCancelled() {
		return nil, errors.New("chat is disabled for cancelled programmes")
	}

	// Verify rate limit
	allowed, warning := CheckChatRateLimit(programmeID, userID)
	if !allowed {
		return nil, errors.New(warning)
	}

	msg := &models.ProgrammeChatMessage{
		ProgrammeID: programmeID,
		UserID:      userID,
		Message:     cleanMsg,
		IsHidden:    false,
		CreatedAt:   time.Now(),
	}

	if err := repository.CreateProgrammeChatMessage(msg); err != nil {
		return nil, err
	}

	// Update in-memory rate-limit map
	RecordChatMessageTime(programmeID, userID)

	// Populate sender user info
	u, err := repository.GetUserByID(userID)
	if err == nil {
		msg.User = &u
	}

	return msg, nil
}

// SubmitProgrammePrayerRequest validates and creates a prayer request for a programme.
func SubmitProgrammePrayerRequest(programmeID, userID int, requestText string, isPrivate bool) (*models.ProgrammePrayerRequest, error) {
	cleanReq := strings.TrimSpace(requestText)
	if len(cleanReq) < 3 {
		return nil, errors.New("prayer request must be at least 3 characters")
	}
	if len(cleanReq) > 1000 {
		return nil, errors.New("prayer request exceeds maximum length of 1000 characters")
	}

	p, err := repository.GetProgrammeByID(programmeID)
	if err != nil {
		return nil, errors.New("programme not found")
	}
	if p.IsCancelled() {
		return nil, errors.New("prayer requests are disabled for cancelled programmes")
	}

	req := &models.ProgrammePrayerRequest{
		ProgrammeID: programmeID,
		UserID:      userID,
		Request:     cleanReq,
		IsPrivate:   isPrivate,
		Status:      models.PrayerStatusPending,
		CreatedAt:   time.Now(),
	}

	if err := repository.CreateProgrammePrayerRequest(req); err != nil {
		return nil, err
	}

	u, err := repository.GetUserByID(userID)
	if err == nil {
		req.User = &u
	}

	return req, nil
}

package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Conversation represents a direct or group chat thread.
type Conversation struct {
	ID                   int
	Type                 string // "direct" or "group"
	Title                string // Used for group chat
	Category             string // "general", "bible_study", "prayer", "evangelism", "other"
	Description          string
	AvatarURL            string
	CallPermission       string // "all", "admin_only"
	IsPublic             bool
	JoinPolicy           string // "approval", "open"
	DisappearingDuration int    // Expiry timer in seconds: 0 = Off, 86400 = 24h, 604800 = 7d, 7776000 = 90d
	CreatedBy            int
	LastMessageAt        *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// HasDisappearingMessages returns true if messages in this conversation expire.
func (c Conversation) HasDisappearingMessages() bool {
	return c.DisappearingDuration > 0
}

// FormattedDisappearingDuration returns a human-friendly label for the expiring timer.
func (c Conversation) FormattedDisappearingDuration() string {
	switch c.DisappearingDuration {
	case 86400:
		return "24 hours"
	case 604800:
		return "7 days"
	case 7776000:
		return "90 days"
	default:
		if c.DisappearingDuration > 0 {
			hours := c.DisappearingDuration / 3600
			if hours >= 24 && hours%24 == 0 {
				return fmt.Sprintf("%d days", hours/24)
			}
			return fmt.Sprintf("%d hours", hours)
		}
		return "Off"
	}
}

// ConversationMember represents a user's participation in a conversation.
type ConversationMember struct {
	ID                int
	ConversationID    int
	UserID            int
	Role              string // "owner", "admin", "moderator", "member"
	LastReadMessageID int
	LastReadAt        *time.Time
	IsMuted           bool
	MutedUntil        *time.Time
	IsArchived        bool
	IsPinned          bool
	ClearedAt         *time.Time
	JoinedAt          time.Time

	// Hydrated user details
	User User
}

// IsActiveMuted evaluates whether the conversation member is currently muted (considering expiration).
func (cm ConversationMember) IsActiveMuted() bool {
	if !cm.IsMuted {
		return false
	}
	if cm.MutedUntil == nil {
		return true // Indefinite
	}
	return cm.MutedUntil.After(time.Now())
}

// FormattedMutedUntil returns a human-readable description of mute duration.
func (cm ConversationMember) FormattedMutedUntil() string {
	if !cm.IsMuted {
		return "Not muted"
	}
	if cm.MutedUntil == nil {
		return "Until unmuted"
	}
	remaining := time.Until(*cm.MutedUntil)
	if remaining <= 0 {
		return "Expired"
	}
	if remaining < time.Hour {
		mins := int(remaining.Minutes())
		if mins <= 1 {
			return "for 1 min"
		}
		return fmt.Sprintf("for %d mins", mins)
	}
	if remaining < 24*time.Hour {
		hours := int(remaining.Hours())
		if hours <= 1 {
			return "for 1 hour"
		}
		return fmt.Sprintf("for %d hours", hours)
	}
	days := int(remaining.Hours() / 24)
	if days <= 1 {
		return "for 1 day"
	}
	return fmt.Sprintf("for %d days", days)
}

// Message represents a single chat message.
type Message struct {
	ID                int
	ConversationID    int
	SenderID          int
	MessageType       string // "text", "image", "voice", "file", "system"
	Body              string
	ClientMessageID   string
	ReplyToMessageID  *int
	IsEdited          bool
	IsDeleted         bool
	DeletedBy         *int
	DeletedAt         *time.Time
	IsReadByRecipient bool // True if recipient has read this message
	AttachmentURL     string
	AttachmentName    string
	AttachmentSize    int64
	AttachmentType    string
	ExpiresAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// Hydrated sender details
	Sender User

	// Quoted reply details
	ReplyToMessage *Message
	IsDeletedForMe bool
}

// IsExpired returns true if the message has an expiration time in the past.
func (m Message) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return m.ExpiresAt.Before(time.Now())
}

// HasExpiration returns true if the message has a set expiration time.
func (m Message) HasExpiration() bool {
	return m.ExpiresAt != nil
}

// FormattedRemainingTime returns human-readable remaining time before disappearing.
func (m Message) FormattedRemainingTime() string {
	if m.ExpiresAt == nil {
		return ""
	}
	remaining := time.Until(*m.ExpiresAt)
	if remaining <= 0 {
		return "Expired"
	}
	if remaining < time.Minute {
		return "<1m"
	}
	if remaining < time.Hour {
		return fmt.Sprintf("%dm", int(remaining.Minutes()))
	}
	if remaining < 24*time.Hour {
		return fmt.Sprintf("%dh", int(remaining.Hours()))
	}
	return fmt.Sprintf("%dd", int(remaining.Hours()/24))
}

// FormattedAttachmentSize returns a human-readable file size string.
func (m Message) FormattedAttachmentSize() string {
	b := m.AttachmentSize
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

// AttachmentExtension returns the lowercase file extension without leading dot.
func (m Message) AttachmentExtension() string {
	ext := strings.ToLower(filepath.Ext(m.AttachmentName))
	return strings.TrimPrefix(ext, ".")
}

// FileIconClass returns an appropriate Bootstrap icon class for the file type.
func (m Message) FileIconClass() string {
	ext := m.AttachmentExtension()
	switch ext {
	case "pdf":
		return "bi bi-file-earmark-pdf-fill text-danger"
	case "doc", "docx":
		return "bi bi-file-earmark-word-fill text-primary"
	case "xls", "xlsx", "csv":
		return "bi bi-file-earmark-excel-fill text-success"
	case "ppt", "pptx":
		return "bi bi-file-earmark-ppt-fill text-warning"
	case "zip", "rar", "tar", "gz", "7z":
		return "bi bi-file-earmark-zip-fill text-warning"
	case "txt", "log", "md", "json", "xml", "html", "js", "go", "py":
		return "bi bi-file-earmark-code-fill text-secondary"
	case "mp3", "wav", "ogg", "aac", "m4a":
		return "bi bi-file-earmark-music-fill text-info"
	case "mp4", "mov", "avi", "mkv", "webm":
		return "bi bi-file-earmark-play-fill text-danger"
	default:
		return "bi bi-file-earmark-fill text-primary"
	}
}

// CanPreview returns true if the attachment can be viewed/previewed directly in the browser.
func (m Message) CanPreview() bool {
	ext := m.AttachmentExtension()
	return ext == "pdf" || ext == "txt"
}

// IsImage returns true if the message is an image attachment.
func (m Message) IsImage() bool {
	if m.MessageType == "image" || strings.HasPrefix(m.AttachmentType, "image/") {
		return true
	}
	switch m.AttachmentExtension() {
	case "jpg", "jpeg", "png", "webp", "gif":
		return true
	}
	return false
}

// IsVideo returns true if the message is a video attachment.
func (m Message) IsVideo() bool {
	if m.MessageType == "video" || strings.HasPrefix(m.AttachmentType, "video/") {
		return true
	}
	switch m.AttachmentExtension() {
	case "mp4", "mov", "avi", "mkv":
		return true
	}
	return false
}

// IsVoice returns true if the message is a voice note or audio attachment.
func (m Message) IsVoice() bool {
	if m.MessageType == "voice" || strings.HasPrefix(m.AttachmentType, "audio/") {
		return true
	}
	switch m.AttachmentExtension() {
	case "mp3", "wav", "ogg", "aac", "m4a", "webm":
		return true
	}
	return false
}

// IsDocument returns true if the message is a file/document attachment.
func (m Message) IsDocument() bool {
	return m.AttachmentURL != "" && !m.IsImage() && !m.IsVideo() && !m.IsVoice()
}

// HasLink returns true if the message body contains an HTTP or HTTPS URL.
func (m Message) HasLink() bool {
	return strings.Contains(m.Body, "http://") || strings.Contains(m.Body, "https://")
}

// SharedMediaOverview groups attachments into distinct categories for offcanvas display.
type SharedMediaOverview struct {
	Photos    []Message
	Videos    []Message
	Documents []Message
	Voice     []Message
	Total     int
}

// TotalCount returns the total number of shared media items.
func (s *SharedMediaOverview) TotalCount() int {
	if s == nil {
		return 0
	}
	return s.Total
}

// PhotoCount returns the number of shared photos.
func (s *SharedMediaOverview) PhotoCount() int {
	if s == nil {
		return 0
	}
	return len(s.Photos)
}

// VideoCount returns the number of shared videos.
func (s *SharedMediaOverview) VideoCount() int {
	if s == nil {
		return 0
	}
	return len(s.Videos)
}

// DocCount returns the number of shared documents.
func (s *SharedMediaOverview) DocCount() int {
	if s == nil {
		return 0
	}
	return len(s.Documents)
}

// VoiceCount returns the number of shared voice notes.
func (s *SharedMediaOverview) VoiceCount() int {
	if s == nil {
		return 0
	}
	return len(s.Voice)
}

// VoiceNotes returns the list of voice note messages.
func (s *SharedMediaOverview) VoiceNotes() []Message {
	if s == nil {
		return nil
	}
	return s.Voice
}

// ConversationItem represents an inbox list entry with summary info.
type ConversationItem struct {
	ID                      int
	Type                    string
	Title                   string
	Category                string
	Description             string
	AvatarURL               string
	CallPermission          string
	IsPublic                bool
	JoinPolicy              string // "approval", "open"
	MembershipRequestStatus string // "none", "pending", "approved", "rejected", "cancelled"
	MembershipRequestID     int
	MemberCount             int
	OtherUser               User
	LastMessage             string
	LastMessageSenderID     int
	LastMessageTime         time.Time
	UnreadCount             int
	IsActive                bool
	IsMuted                 bool
	MutedUntil              *time.Time
	DisappearingDuration    int
	IsArchived              bool
	IsPinned                bool
}

// MessageDeletion represents a user-specific "Delete for Me" action.
type MessageDeletion struct {
	ID        int       `json:"id"`
	MessageID int       `json:"message_id"`
	UserID    int       `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// GroupMembershipRequest represents a user's request to join a community group requiring approval.
type GroupMembershipRequest struct {
	ID             int        `json:"id"`
	ConversationID int        `json:"conversation_id"`
	UserID         int        `json:"user_id"`
	Status         string     `json:"status"` // "pending", "approved", "rejected", "cancelled"
	RequestMessage string     `json:"request_message"`
	ReviewedBy     *int       `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Hydrated details
	User     User  `json:"user"`
	Reviewer *User `json:"reviewer,omitempty"`
}

// ChatViewData holds template data for the chat interface.
type ChatViewData struct {
	CurrentUser                User
	Error                      string
	Conversations              []ConversationItem
	ActiveConversation         *Conversation
	ActiveRecipient            *User
	ActiveGroupMembers         []ConversationMember
	ActiveUserRole             string // "owner", "admin", "moderator", "member"
	CanStartCall               bool
	CanManageMembers           bool
	DiscoverableGroups         []ConversationItem
	Messages                   []Message
	EligibleFriends            []User
	UnreadCount                int
	UnreadMessagesCount        int
	IsActiveMuted              bool
	ActiveMutedUntil           *time.Time
	ActiveDisappearingDuration int
	ActiveDisappearingLabel    string
	IsActiveArchived           bool
	IsActivePinned             bool
	HasOlderMessages           bool
	IsActiveOnline             bool
	IsRecipientBlocked         bool
	IsBlockedByRecipient       bool
	SharedMedia                []Message
	SharedMediaOverview        *SharedMediaOverview
	CallLogs                   []CallLog
	ActiveGroupCall            *GroupCallRoom
	PendingRequestsCount       int
	PendingRequests            []GroupMembershipRequest
	BlockedUsers               []User
	PendingReportsCount        int
	HasAcceptedTerms           bool
	ActiveTermsVersion         string
}

// CallParticipant represents a participant who joined a call session.
type CallParticipant struct {
	ID              int        `json:"id"`
	CallID          int        `json:"call_id"`
	UserID          int        `json:"user_id"`
	JoinedAt        time.Time  `json:"joined_at"`
	LeftAt          *time.Time `json:"left_at,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`

	// Hydrated details
	User User `json:"user,omitempty"`
}

// GroupCallRoom represents an active multi-party audio or video call session in memory.
type GroupCallRoom struct {
	CallID         int                          `json:"call_id"`
	ConversationID int                          `json:"conversation_id"`
	InitiatorID    int                          `json:"initiator_id"`
	InitiatorName  string                       `json:"initiator_name"`
	CallType       string                       `json:"call_type"` // "audio" or "video"
	StartedAt      time.Time                    `json:"started_at"`
	Participants   map[int]*CallParticipantInfo `json:"participants"`
}

// CallParticipantInfo holds live in-call state for an active participant.
type CallParticipantInfo struct {
	UserID       int       `json:"user_id"`
	FullName     string    `json:"full_name"`
	Username     string    `json:"username"`
	ProfilePhoto string    `json:"profile_photo"`
	Role         string    `json:"role"` // "owner", "admin", "moderator", "member"
	JoinedAt     time.Time `json:"joined_at"`
	IsMuted      bool      `json:"is_muted"`
	IsCamOff     bool      `json:"is_cam_off"`
	IsSpeaking   bool      `json:"is_speaking"`
}

// BlockedUser represents a blocked member relationship.
type BlockedUser struct {
	ID        int       `json:"id"`
	BlockerID int       `json:"blocker_id"`
	BlockedID int       `json:"blocked_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`

	// Hydrated blocked user
	User User `json:"user"`
}

// ChatReport represents an abuse or moderation report submitted against a user or message.
type ChatReport struct {
	ID               int        `json:"id"`
	ReporterID       int        `json:"reporter_id"`
	ReportedUserID   int        `json:"reported_user_id"`
	ConversationID   *int       `json:"conversation_id,omitempty"`
	MessageID        *int       `json:"message_id,omitempty"`
	Reason           string     `json:"reason"`
	Details          string     `json:"details"`
	Status           string     `json:"status"` // "pending", "reviewed", "dismissed", "action_taken"
	EvidenceText     string     `json:"evidence_text,omitempty"`
	EvidenceURL      string     `json:"evidence_url,omitempty"`
	ReviewedBy       *int       `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	ResolutionNotes  string     `json:"resolution_notes,omitempty"`
	ResolutionAction string     `json:"resolution_action,omitempty"` // "none", "warning_issued", "message_deleted", "user_suspended"
	CreatedAt        time.Time  `json:"created_at"`

	// Hydrated details
	Reporter     *User    `json:"reporter,omitempty"`
	ReportedUser *User    `json:"reported_user,omitempty"`
	Reviewer     *User    `json:"reviewer,omitempty"`
	Message      *Message `json:"message,omitempty"`
}

// CallLog represents a record of an audio or video call session.
type CallLog struct {
	ID              int        `json:"id"`
	ConversationID  int        `json:"conversation_id"`
	CallerID        int        `json:"caller_id"`
	ReceiverID      int        `json:"receiver_id"`
	CallType        string     `json:"call_type"` // "audio" or "video"
	Status          string     `json:"status"`    // "initiated", "completed", "missed", "declined", "busy", "failed"
	DurationSeconds int        `json:"duration_seconds"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`

	// Hydrated details
	Caller       User              `json:"caller,omitempty"`
	Receiver     User              `json:"receiver,omitempty"`
	Participants []CallParticipant `json:"participants,omitempty"`
}

// CallInvitation represents a shareable invitation token to an audio or video call session.
type CallInvitation struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	Token          string    `json:"token"`
	CallType       string    `json:"call_type"` // "audio" or "video"
	CreatedBy      int       `json:"created_by"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

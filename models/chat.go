package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Conversation represents a direct or group chat thread.
type Conversation struct {
	ID             int
	Type           string // "direct" or "group"
	Title          string // Used for group chat
	Category       string // "general", "bible_study", "prayer", "evangelism", "other"
	Description    string
	AvatarURL      string
	CallPermission string // "all", "admin_only"
	IsPublic       bool
	JoinPolicy     string // "approval", "open"
	CreatedBy      int
	LastMessageAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
	IsArchived        bool
	JoinedAt          time.Time

	// Hydrated user details
	User User
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
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// Hydrated sender details
	Sender User
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
	IsArchived              bool
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
	CurrentUser          User
	Error                string
	Conversations        []ConversationItem
	ActiveConversation   *Conversation
	ActiveRecipient      *User
	ActiveGroupMembers   []ConversationMember
	ActiveUserRole       string // "owner", "admin", "moderator", "member"
	CanStartCall         bool
	CanManageMembers     bool
	DiscoverableGroups   []ConversationItem
	Messages             []Message
	EligibleFriends      []User
	UnreadCount          int
	UnreadMessagesCount  int
	IsActiveMuted        bool
	IsActiveArchived     bool
	HasOlderMessages     bool
	IsActiveOnline       bool
	IsRecipientBlocked   bool
	IsBlockedByRecipient bool
	SharedMedia          []Message
	CallLogs             []CallLog
	ActiveGroupCall      *GroupCallRoom
	PendingRequestsCount int
	PendingRequests      []GroupMembershipRequest
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
	ID             int       `json:"id"`
	ReporterID     int       `json:"reporter_id"`
	ReportedUserID int       `json:"reported_user_id"`
	ConversationID *int      `json:"conversation_id"`
	MessageID      *int      `json:"message_id"`
	Reason         string    `json:"reason"`
	Details        string    `json:"details"`
	Status         string    `json:"status"` // "pending", "reviewed", "dismissed", "action_taken"
	CreatedAt      time.Time `json:"created_at"`
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

	// Hydrated details
	Caller       User              `json:"caller,omitempty"`
	Receiver     User              `json:"receiver,omitempty"`
	Participants []CallParticipant `json:"participants,omitempty"`
}

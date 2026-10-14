package models

import "time"

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

// ConversationItem represents an inbox list entry with summary info.
type ConversationItem struct {
	ID                  int
	Type                string
	Title               string
	Category            string
	Description         string
	AvatarURL           string
	CallPermission      string
	IsPublic            bool
	MemberCount         int
	OtherUser           User
	LastMessage         string
	LastMessageSenderID int
	LastMessageTime     time.Time
	UnreadCount         int
	IsActive            bool
	IsMuted             bool
	IsArchived          bool
}

// ChatViewData holds template data for the chat interface.
type ChatViewData struct {
	CurrentUser          User
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




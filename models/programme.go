package models

import (
	"fmt"
	"time"
)

// Programme Statuses
const (
	ProgrammeStatusDraft     = "draft"
	ProgrammeStatusScheduled = "scheduled"
	ProgrammeStatusLive      = "live"
	ProgrammeStatusEnded     = "ended"
	ProgrammeStatusCancelled = "cancelled"
)

// Programme Visibilities
const (
	ProgrammeVisibilityPublic      = "public"
	ProgrammeVisibilityMembersOnly = "members_only"
)

// Programme Categories
const (
	ProgrammeCategoryService         = "service"
	ProgrammeCategoryPrayer          = "prayer"
	ProgrammeCategoryBibleStudy      = "bible_study"
	ProgrammeCategoryYouthFellowship = "youth_fellowship"
	ProgrammeCategoryConvention      = "convention"
	ProgrammeCategorySpecial         = "special"
)

// Prayer Request Statuses
const (
	PrayerStatusPending  = "pending"
	PrayerStatusPraying  = "praying"
	PrayerStatusAnswered = "answered"
)

// Programme represents a live streaming or online event session.
type Programme struct {
	ID                 int
	Title              string
	Description        string
	Category           string
	OrganizerID        int
	Organizer          *User
	ScheduleStartTime  time.Time
	ScheduleEndTime    time.Time
	Timezone           string
	Visibility         string
	YouTubeVideoID     string
	Status             string
	CancellationReason string
	RecordingAvailable bool
	RecordingVideoID   string
	CreatedAt          time.Time
	UpdatedAt          time.Time

	// View-only / computed helper fields
	CanManage   bool
	IsModerator bool
}

func (p *Programme) IsLive() bool {
	return p.Status == ProgrammeStatusLive
}

func (p *Programme) IsScheduled() bool {
	return p.Status == ProgrammeStatusScheduled
}

func (p *Programme) IsEnded() bool {
	return p.Status == ProgrammeStatusEnded
}

func (p *Programme) IsCancelled() bool {
	return p.Status == ProgrammeStatusCancelled
}

func (p *Programme) IsMembersOnly() bool {
	return p.Visibility == ProgrammeVisibilityMembersOnly
}

// ActiveVideoID returns the recording ID when ended (if available), otherwise the live stream ID.
func (p *Programme) ActiveVideoID() string {
	if p.IsEnded() && p.RecordingAvailable {
		if p.RecordingVideoID != "" {
			return p.RecordingVideoID
		}
		return p.YouTubeVideoID
	}
	return p.YouTubeVideoID
}

// EmbedURL returns the privacy-enhanced YouTube embed link.
func (p *Programme) EmbedURL() string {
	vid := p.ActiveVideoID()
	if vid == "" {
		return ""
	}
	return fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s?rel=0&modestbranding=1", vid)
}

// CategoryDisplay returns a human-friendly string for the category.
func (p *Programme) CategoryDisplay() string {
	switch p.Category {
	case ProgrammeCategoryService:
		return "Church Service"
	case ProgrammeCategoryPrayer:
		return "Prayer Meeting"
	case ProgrammeCategoryBibleStudy:
		return "Bible Study"
	case ProgrammeCategoryYouthFellowship:
		return "Youth Fellowship"
	case ProgrammeCategoryConvention:
		return "Convention"
	case ProgrammeCategorySpecial:
		return "Special Event"
	default:
		return "Programme"
	}
}

// StatusBadgeClass returns the Bootstrap CSS badge class for the current status.
func (p *Programme) StatusBadgeClass() string {
	switch p.Status {
	case ProgrammeStatusLive:
		return "bg-danger"
	case ProgrammeStatusScheduled:
		return "bg-primary"
	case ProgrammeStatusEnded:
		return "bg-secondary"
	case ProgrammeStatusCancelled:
		return "bg-dark"
	default:
		return "bg-warning text-dark"
	}
}

// ProgrammeModerator represents a moderator assigned to a programme.
type ProgrammeModerator struct {
	ID          int
	ProgrammeID int
	UserID      int
	User        *User
	AssignedAt  time.Time
}

// ProgrammeChatMessage represents a live chat message sent during a programme.
type ProgrammeChatMessage struct {
	ID          int
	ProgrammeID int
	UserID      int
	User        *User
	Message     string
	IsHidden    bool
	CreatedAt   time.Time
}

// ProgrammePrayerRequest represents an online prayer request submitted for a programme.
type ProgrammePrayerRequest struct {
	ID          int
	ProgrammeID int
	UserID      int
	User        *User
	Request     string
	IsPrivate   bool
	Status      string
	CreatedAt   time.Time

	// View helper
	CanManageStatus bool
}

// ProgrammeNotificationLog records notifications sent to avoid duplicates.
type ProgrammeNotificationLog struct {
	ID               int
	ProgrammeID      int
	NotificationType string
	SentAt           time.Time
}

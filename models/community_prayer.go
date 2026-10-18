package models

import "time"

// CommunityPrayerRequest represents an intercessory prayer request submitted in the youth community
type CommunityPrayerRequest struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	AuthorName     string    `json:"author_name"`
	AuthorUsername string    `json:"author_username"`
	AuthorAvatar   string    `json:"author_avatar"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	PrivacyScope   string    `json:"privacy_scope"` // 'public', 'fellowship', 'pastoral', 'private'
	PrayCount      int       `json:"pray_count"`
	Status         string    `json:"status"` // 'active', 'answered', 'archived'
	CreatedAt      time.Time `json:"created_at"`
	HasPrayed      bool      `json:"has_prayed"`
	IsOwner        bool      `json:"is_owner"`
}

// CommunityStats represents aggregate metrics across the youth community
type CommunityStats struct {
	MembersCount   int `json:"members_count"`
	PostsCount     int `json:"posts_count"`
	ReactionsCount int `json:"reactions_count"`
	CommentsCount  int `json:"comments_count"`
}

// WeeklyBibleStudy represents the current week's scripture topic and reflection material
type WeeklyBibleStudy struct {
	Theme               string   `json:"theme"`
	ScripturePassage    string   `json:"scripture_passage"`
	MemoryVerse         string   `json:"memory_verse"`
	MemoryVerseText     string   `json:"memory_verse_text"`
	DiscussionQuestions []string `json:"discussion_questions"`
	StudyGroupLink      string   `json:"study_group_link"`
}


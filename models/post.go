package models

import "time"

type Post struct {
	ID int

	UserID int

	Content string

	Image string

	Video string

	Visibility string

	Likes int

	Comments int

	Shares int

	IsEdited bool

	CreatedAt time.Time

	UpdatedAt time.Time

	User User

	// Community features
	ReactionSummary ReactionSummary

	CommentsList []Comment

	// Enhanced reactions
	UserReaction      string
	UserReactionEmoji string
	UserReactionLabel string
	ReactionCounts    map[string]int
	TotalReactions    int
	TopReactions      []ReactionCountItem
}

// ReactionCountItem models a reaction chip showing emoji and its count.
type ReactionCountItem struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

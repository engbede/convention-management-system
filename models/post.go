package models

import (
	"os"
	"strings"
	"time"
)

func getMediaBaseURL() string {
	for _, env := range []string{"S3_PUBLIC_URL", "MEDIA_BASE_URL", "CLOUDINARY_URL", "CDN_URL"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			return strings.TrimSuffix(val, "/")
		}
	}
	return ""
}

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

// ImageURL returns the full, browser-loadable URL for the post image.
// Supports external URLs (Cloudinary/S3/CDN), already-prefixed paths, and local upload paths.
func (p Post) ImageURL() string {
	if p.Image == "" {
		return ""
	}
	if strings.HasPrefix(p.Image, "http://") || strings.HasPrefix(p.Image, "https://") {
		return p.Image
	}
	if cdn := getMediaBaseURL(); cdn != "" {
		rel := p.Image
		if !strings.HasPrefix(rel, "posts/") && !strings.HasPrefix(rel, "/uploads/posts/") {
			rel = "posts/" + strings.TrimPrefix(rel, "/")
		}
		rel = strings.TrimPrefix(rel, "/uploads/")
		return cdn + "/" + rel
	}
	rel := strings.TrimPrefix(p.Image, "/uploads/")
	rel = strings.TrimPrefix(rel, "/")
	if !strings.HasPrefix(rel, "posts/") {
		rel = "posts/" + rel
	}
	return "/uploads/" + rel
}

// VideoURL returns the full, browser-loadable URL for the post video.
// Supports external URLs (Cloudinary/S3/CDN), already-prefixed paths, and local upload paths.
func (p Post) VideoURL() string {
	if p.Video == "" {
		return ""
	}
	if strings.HasPrefix(p.Video, "http://") || strings.HasPrefix(p.Video, "https://") {
		return p.Video
	}
	if cdn := getMediaBaseURL(); cdn != "" {
		rel := p.Video
		if !strings.HasPrefix(rel, "posts/") && !strings.HasPrefix(rel, "/uploads/posts/") {
			rel = "posts/" + strings.TrimPrefix(rel, "/")
		}
		rel = strings.TrimPrefix(rel, "/uploads/")
		return cdn + "/" + rel
	}
	rel := strings.TrimPrefix(p.Video, "/uploads/")
	rel = strings.TrimPrefix(rel, "/")
	if !strings.HasPrefix(rel, "posts/") {
		rel = "posts/" + rel
	}
	return "/uploads/" + rel
}


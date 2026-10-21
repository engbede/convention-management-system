package models

import (
	"os"
	"path/filepath"
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

// VideoMimeType returns the browser MIME type corresponding to the video URL for HTML5 video playback.
func (p Post) VideoMimeType() string {
	rawURL := strings.ToLower(p.VideoURL())
	clean := strings.Split(rawURL, "?")[0]
	ext := filepath.Ext(clean)
	switch ext {
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "video/mp4"
	}
}

// IsWebPlayableVideo returns true if the video container is natively supported by modern web browsers.
// Containers like MKV and AVI are not supported in HTML5 video tags and require external players or conversion.
func (p Post) IsWebPlayableVideo() bool {
	rawURL := strings.ToLower(p.VideoURL())
	clean := strings.Split(rawURL, "?")[0]
	ext := filepath.Ext(clean)
	return ext == ".mp4" || ext == ".webm" || ext == ".mov" || ext == ""
}



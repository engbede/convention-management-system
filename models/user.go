package models

import (
	"strings"
	"time"
)

type User struct {
	// Identity
	ID       int
	FullName string
	Username string

	// View State
	IsOwner bool

	// Contact
	Email string
	Phone string

	// Authentication
	PasswordHash string
	Role         string

	// Basic Profile
	Bio        string
	Gender     string
	BirthDate  string
	Occupation string

	Location string
	State    string
	Country  string

	Website      string
	ProfilePhoto string
	CoverPhoto   string
	Initials     string

	// Church Information
	ChurchName  string
	Circuit     string
	LocalChurch string
	Department  string
	Position    string

	// Spiritual Profile
	FavoriteBibleVerse string
	LifeVerse          string
	SalvationTestimony string
	Calling            string
	SpiritualGifts     string

	WaterBaptized      bool
	HolySpiritBaptized bool

	// Interests
	FavoritePreacher      string
	FavoriteChristianBook string
	FavoriteWorshipSong   string
	FavoriteGospelArtist  string

	Hobbies   string
	Skills    string
	Languages string

	// Vision
	Mission       string
	Vision        string
	FavoriteQuote string

	// Account Status
	IsVerified bool
	IsActive   bool

	// Social
	Followers int
	Following int

	PostsCount     int
	FollowersCount int
	FollowingCount int
	FriendsCount   int

	IsFollowing  bool
	FriendStatus string

	// Statistics
	PrayerRequestsCount int
	BibleStudiesCount   int
	TestimoniesCount    int

	// Dates
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProfilePhotoURL returns the browser-loadable URL for the user's profile photo.
// Supports external URLs (Cloudinary/S3/CDN), already-prefixed paths, and local upload paths.
func (u User) ProfilePhotoURL() string {
	if u.ProfilePhoto == "" {
		return ""
	}
	if strings.HasPrefix(u.ProfilePhoto, "http://") || strings.HasPrefix(u.ProfilePhoto, "https://") {
		return u.ProfilePhoto
	}
	if cdn := getMediaBaseURL(); cdn != "" {
		rel := u.ProfilePhoto
		if !strings.HasPrefix(rel, "profiles/") && !strings.HasPrefix(rel, "/uploads/profiles/") {
			rel = "profiles/" + strings.TrimPrefix(rel, "/")
		}
		rel = strings.TrimPrefix(rel, "/uploads/")
		return cdn + "/" + rel
	}
	rel := strings.TrimPrefix(u.ProfilePhoto, "/uploads/")
	rel = strings.TrimPrefix(rel, "/")
	if !strings.HasPrefix(rel, "profiles/") {
		rel = "profiles/" + rel
	}
	return "/uploads/" + rel
}

// CoverPhotoURL returns the browser-loadable URL for the user's cover photo.
// Supports external URLs (Cloudinary/S3/CDN), already-prefixed paths, and local upload paths.
func (u User) CoverPhotoURL() string {
	if u.CoverPhoto == "" {
		return ""
	}
	if strings.HasPrefix(u.CoverPhoto, "http://") || strings.HasPrefix(u.CoverPhoto, "https://") {
		return u.CoverPhoto
	}
	if cdn := getMediaBaseURL(); cdn != "" {
		rel := u.CoverPhoto
		if !strings.HasPrefix(rel, "covers/") && !strings.HasPrefix(rel, "/uploads/covers/") {
			rel = "covers/" + strings.TrimPrefix(rel, "/")
		}
		rel = strings.TrimPrefix(rel, "/uploads/")
		return cdn + "/" + rel
	}
	rel := strings.TrimPrefix(u.CoverPhoto, "/uploads/")
	rel = strings.TrimPrefix(rel, "/")
	if !strings.HasPrefix(rel, "covers/") {
		rel = "covers/" + rel
	}
	return "/uploads/" + rel
}

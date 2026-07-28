package models

import "time"

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

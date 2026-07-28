package models

type ProfilePage struct {
	CurrentUser User
	User        User

	Posts []Post

	Friends []Friend

	Notifications []Notification

	UnreadNotifications int
}

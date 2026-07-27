package services

import "convention-management-system/repository"

func FollowUser(followerID, followingID int) error {
	return repository.FollowUser(followerID, followingID)
}

func UnfollowUser(followerID, followingID int) error {
	return repository.UnfollowUser(followerID, followingID)
}

func GetFollowersCount(userID int) (int, error) {
	return repository.GetFollowersCount(userID)
}

func GetFollowingCount(userID int) (int, error) {
	return repository.GetFollowingCount(userID)
}

func IsFollowing(followerID, followingID int) (bool, error) {
	return repository.IsFollowing(followerID, followingID)
}

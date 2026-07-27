package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func GetUserProfile(userID int) (*models.User, error) {

	user, err := repository.GetUserProfile(userID)
	if err != nil {
		return nil, err
	}

	followers, _ := repository.GetFollowersCount(userID)
	following, _ := repository.GetFollowingCount(userID)

	user.FollowersCount = followers
	user.FollowingCount = following

	return user, nil
}

func UpdateUserProfile(user *models.User) error {
	return repository.UpdateUserProfile(user)
}

func UpdateProfilePhoto(
	userID int,
	filename string,
) error {

	return repository.UpdateProfilePhoto(
		userID,
		filename,
	)
}

func UpdateCoverPhoto(
	userID int,
	filename string,
) error {

	return repository.UpdateCoverPhoto(
		userID,
		filename,
	)
}

func GetProfilePhotos(userID int) (string, string, error) {
	return repository.GetProfilePhotos(userID)
}

func GetUserByUsername(username string) (*models.User, error) {

	user, err := repository.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	followers, _ := repository.GetFollowersCount(user.ID)
	following, _ := repository.GetFollowingCount(user.ID)

	user.FollowersCount = followers
	user.FollowingCount = following

	return user, nil
}

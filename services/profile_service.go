package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func GetUserProfile(userID int) (*models.User, error) {
	return repository.GetUserProfile(userID)
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

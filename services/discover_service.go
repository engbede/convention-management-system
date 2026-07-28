package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func GetDiscoverUsers(currentUserID int) ([]models.User, error) {
	return repository.GetDiscoverUsers(currentUserID)
}

package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func CreateComment(
	comment *models.Comment,
) error {

	return repository.CreateComment(comment)
}

func GetCommentsByPost(
	postID int,
) ([]models.Comment, error) {

	return repository.GetCommentsByPost(postID)
}

func CountComments(
	postID int,
) (int, error) {

	return repository.CountComments(postID)
}

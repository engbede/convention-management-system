package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func GetCommunityFeed() ([]models.Post, error) {

	return repository.GetAllPosts()
}

func CreatePost(
	post *models.Post,
) error {

	return repository.CreatePost(post)
}

func GetPostsByUser(userID int) ([]models.Post, error) {
	return repository.GetPostsByUser(userID)
}

// func DeletePost(
// 	postID int,
// 	userID int,
// ) error {

// 	return repository.DeletePost(
// 		postID,
// 		userID,
// 	)
// }

// func UpdatePost(
// 	post models.Post,
// ) error {

// 	return repository.UpdatePost(post)
// }

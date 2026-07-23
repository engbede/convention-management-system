package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

// CreateComment inserts a new comment.
func CreateComment(
	comment *models.Comment,
) error {

	query := `
	INSERT INTO comments (
		post_id,
		user_id,
		parent_id,
		content
	)
	VALUES (?, ?, ?, ?)
	`

	_, err := database.DB.Exec(
		query,
		comment.PostID,
		comment.UserID,
		comment.ParentID,
		comment.Content,
	)

	return err
}

// GetCommentsByPost returns all top-level comments for a post.
func GetCommentsByPost(
	postID int,
) ([]models.Comment, error) {

	query := `
	SELECT
		c.id,
		c.post_id,
		c.user_id,
		c.parent_id,
		c.content,
		c.is_edited,
		c.created_at,
		c.updated_at,
		u.full_name,
		u.username,
		u.profile_photo
	FROM comments c
	INNER JOIN users u
		ON c.user_id = u.id
	WHERE c.post_id = ?
	AND c.parent_id IS NULL
	ORDER BY c.created_at ASC
	`

	rows, err := database.DB.Query(
		query,
		postID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var comments []models.Comment

	for rows.Next() {

		var comment models.Comment

		err := rows.Scan(
			&comment.ID,
			&comment.PostID,
			&comment.UserID,
			&comment.ParentID,
			&comment.Content,
			&comment.IsEdited,
			&comment.CreatedAt,
			&comment.UpdatedAt,
			&comment.User.FullName,
			&comment.User.Username,
			&comment.User.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		comments = append(
			comments,
			comment,
		)
	}

	return comments, nil
}

// CountComments returns the number of comments for a post.
func CountComments(
	postID int,
) (int, error) {

	var count int

	query := `
	SELECT COUNT(*)
	FROM comments
	WHERE post_id = ?
	`

	err := database.DB.QueryRow(
		query,
		postID,
	).Scan(&count)

	return count, err
}
package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/utils"
)

func CreatePost(post *models.Post) error {

	query := `
	INSERT INTO posts(
		user_id,
		content,
		image,
		video,
		visibility
	)
	VALUES(?,?,?,?,?)
	`

	_, err := database.DB.Exec(
		query,
		post.UserID,
		post.Content,
		post.Image,
		post.Video,
		post.Visibility,
	)

	return err
}

func GetAllPosts() ([]models.Post, error) {

	query := `
	SELECT
		p.id,
		p.user_id,
		p.content,
		p.image,
		p.video,
		p.visibility,
		p.likes,
		p.comments,
		p.shares,
		p.is_edited,
		p.created_at,
		p.updated_at,
		u.full_name,
		u.username,
		u.profile_photo
	FROM posts p
	INNER JOIN users u
		ON p.user_id = u.id
	ORDER BY p.created_at DESC
	`

	rows, err := database.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post

	for rows.Next() {

		var post models.Post

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.Content,
			&post.Image,
			&post.Video,
			&post.Visibility,
			&post.Likes,
			&post.Comments,
			&post.Shares,
			&post.IsEdited,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.User.FullName,
			&post.User.Username,
			&post.User.ProfilePhoto,
		)

		post.User.Initials = utils.Initials(
			post.User.FullName,
		)
		if err != nil {
			return nil, err
		}
		// Load reaction summary
		summary, err := GetReactionSummary(post.ID)
		if err == nil {
			post.ReactionSummary = summary
			post.Likes = summary.Total
		}

		// Load comments
		comments, err := GetCommentsByPost(post.ID)
		if err == nil {
			post.CommentsList = comments
			post.Comments = len(comments)
		}

		posts = append(posts, post)
	}

	return posts, nil
}

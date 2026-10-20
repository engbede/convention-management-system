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
	VALUES($1, $2, $3, $4, $5)
	`

	res, err := database.DB.Exec(
		query,
		post.UserID,
		post.Content,
		post.Image,
		post.Video,
		post.Visibility,
	)
	if err != nil {
		return err
	}

	if res != nil {
		if id, lErr := res.LastInsertId(); lErr == nil && id > 0 {
			post.ID = int(id)
		}
	}

	return nil
}

func GetAllPosts() ([]models.Post, error) {

	query := `
	SELECT
		p.id,
		p.user_id,
		p.content,
		COALESCE(p.image, ''),
		COALESCE(p.video, ''),
		COALESCE(p.visibility, 'public'),
		COALESCE(p.likes, 0),
		COALESCE(p.comments, 0),
		COALESCE(p.shares, 0),
		COALESCE(p.is_edited, FALSE),
		p.created_at,
		p.updated_at,
		u.full_name,
		u.username,
		COALESCE(u.profile_photo, '')
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
		if err != nil {
			return nil, err
		}

		post.User.Initials = utils.Initials(
			post.User.FullName,
		)
		// Load reaction summary
		summary, err := GetReactionSummary(post.ID)
		if err == nil {
			post.ReactionSummary = summary
			post.Likes = summary.Total
		}
		counts, total, cErr := GetPostReactionCounts(post.ID)
		if cErr == nil {
			post.ReactionCounts = counts
			post.TotalReactions = total
		}
		top, tErr := GetTopReactionCounts(post.ID, 5)
		if tErr == nil {
			post.TopReactions = top
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

func GetPostsByUser(userID int) ([]models.Post, error) {

	query := `
	SELECT
		p.id,
		p.user_id,
		p.content,
		COALESCE(p.image, ''),
		COALESCE(p.video, ''),
		COALESCE(p.visibility, 'public'),
		COALESCE(p.likes, 0),
		COALESCE(p.comments, 0),
		COALESCE(p.shares, 0),
		COALESCE(p.is_edited, FALSE),
		p.created_at,
		p.updated_at,
		u.full_name,
		u.username,
		COALESCE(u.profile_photo, '')
	FROM posts p
	INNER JOIN users u
		ON p.user_id = u.id
	WHERE p.user_id = $1
	ORDER BY p.created_at DESC
	`

	rows, err := database.DB.Query(query, userID)
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
		if err != nil {
			return nil, err
		}

		post.User.Initials = utils.Initials(post.User.FullName)

		summary, err := GetReactionSummary(post.ID)
		if err == nil {
			post.ReactionSummary = summary
			post.Likes = summary.Total
		}
		counts, total, cErr := GetPostReactionCounts(post.ID)
		if cErr == nil {
			post.ReactionCounts = counts
			post.TotalReactions = total
		}
		top, tErr := GetTopReactionCounts(post.ID, 5)
		if tErr == nil {
			post.TopReactions = top
		}

		comments, err := GetCommentsByPost(post.ID)
		if err == nil {
			post.CommentsList = comments
			post.Comments = len(comments)
		}

		posts = append(posts, post)
	}

	return posts, nil
}

package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

// Add a new reaction.
func AddReaction(
	postID int,
	userID int,
	reaction string,
) error {

	query := `
	INSERT INTO reactions (
		post_id,
		user_id,
		reaction
	)
	VALUES (?, ?, ?)
	`

	_, err := database.DB.Exec(
		query,
		postID,
		userID,
		reaction,
	)

	return err
}

// Update an existing reaction.
func UpdateReaction(
	postID int,
	userID int,
	reaction string,
) error {

	query := `
	UPDATE reactions
	SET reaction = ?
	WHERE post_id = ?
	AND user_id = ?
	`

	_, err := database.DB.Exec(
		query,
		reaction,
		postID,
		userID,
	)

	return err
}

// Remove a reaction.
func RemoveReaction(
	postID int,
	userID int,
) error {

	query := `
	DELETE FROM reactions
	WHERE post_id = ?
	AND user_id = ?
	`

	_, err := database.DB.Exec(
		query,
		postID,
		userID,
	)

	return err
}

// Return the user's current reaction.
func GetReaction(
	postID int,
	userID int,
) (string, error) {

	var reaction string

	query := `
	SELECT reaction
	FROM reactions
	WHERE post_id = ?
	AND user_id = ?
	`

	err := database.DB.QueryRow(
		query,
		postID,
		userID,
	).Scan(&reaction)

	return reaction, err
}

// Count a single reaction type.
func CountReactionType(
	postID int,
	reaction string,
) (int, error) {

	var count int

	query := `
	SELECT COUNT(*)
	FROM reactions
	WHERE post_id = ?
	AND reaction = ?
	`

	err := database.DB.QueryRow(
		query,
		postID,
		reaction,
	).Scan(&count)

	return count, err
}

// Return all reaction counts for a post.
func GetReactionSummary(
	postID int,
) (models.ReactionSummary, error) {

	var summary models.ReactionSummary

	var err error

	summary.Like, err = CountReactionType(
		postID,
		"like",
	)

	if err != nil {
		return summary, err
	}

	summary.Love, err = CountReactionType(
		postID,
		"love",
	)

	if err != nil {
		return summary, err
	}

	summary.Amen, err = CountReactionType(
		postID,
		"amen",
	)

	if err != nil {
		return summary, err
	}

	summary.Clap, err = CountReactionType(
		postID,
		"clap",
	)

	if err != nil {
		return summary, err
	}

	summary.Fire, err = CountReactionType(
		postID,
		"fire",
	)

	if err != nil {
		return summary, err
	}

	summary.Celebrate, err = CountReactionType(
		postID,
		"celebrate",
	)

	if err != nil {
		return summary, err
	}

	summary.Total =
		summary.Like +
			summary.Love +
			summary.Amen +
			summary.Clap +
			summary.Fire +
			summary.Celebrate

	return summary, nil
}

// Count all reactions regardless of type.
// This keeps compatibility with the current feed.
func CountReactions(
	postID int,
) (int, error) {

	var total int

	query := `
	SELECT COUNT(*)
	FROM reactions
	WHERE post_id = ?
	`

	err := database.DB.QueryRow(
		query,
		postID,
	).Scan(&total)

	return total, err
}

// Optional helper if you later need all reactions
// belonging to a post.
func GetPostReactions(
	postID int,
) ([]models.Reaction, error) {

	query := `
	SELECT
		id,
		post_id,
		user_id,
		reaction,
		created_at
	FROM reactions
	WHERE post_id = ?
	ORDER BY created_at ASC
	`

	rows, err := database.DB.Query(
		query,
		postID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var reactions []models.Reaction

	for rows.Next() {

		var reaction models.Reaction

		err := rows.Scan(
			&reaction.ID,
			&reaction.PostID,
			&reaction.UserID,
			&reaction.Reaction,
			&reaction.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		reactions = append(
			reactions,
			reaction,
		)
	}

	return reactions, nil
}

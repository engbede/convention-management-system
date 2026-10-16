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

	counts, total, err := GetPostReactionCounts(postID)
	if err != nil {
		return summary, err
	}

	summary.Like = counts["like"]
	summary.Love = counts["love"]
	summary.Amen = counts["amen"]
	summary.Clap = counts["clap"]
	summary.Fire = counts["fire"]
	summary.Celebrate = counts["celebrate"]
	summary.Total = total

	return summary, nil
}

// GetPostReactionCounts returns a map of reaction type to count and total count.
func GetPostReactionCounts(postID int) (map[string]int, int, error) {
	query := `
		SELECT reaction, COUNT(*)
		FROM reactions
		WHERE post_id = ?
		GROUP BY reaction
	`
	rows, err := database.DB.Query(query, postID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	total := 0
	for rows.Next() {
		var rType string
		var count int
		if err := rows.Scan(&rType, &count); err == nil {
			counts[rType] = count
			total += count
		}
	}
	return counts, total, nil
}

// GetTopReactionCounts returns the top reactions with their emoji, label, and counts.
func GetTopReactionCounts(postID int, limit int) ([]models.ReactionCountItem, error) {
	if limit <= 0 {
		limit = 5
	}
	query := `
		SELECT reaction, COUNT(*) as c
		FROM reactions
		WHERE post_id = ?
		GROUP BY reaction
		ORDER BY c DESC, reaction ASC
		LIMIT ?
	`
	rows, err := database.DB.Query(query, postID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.ReactionCountItem
	for rows.Next() {
		var rType string
		var count int
		if err := rows.Scan(&rType, &count); err == nil {
			items = append(items, models.ReactionCountItem{
				Type:  rType,
				Emoji: models.GetReactionEmoji(rType),
				Label: models.GetReactionLabel(rType),
				Count: count,
			})
		}
	}
	return items, nil
}

// GetReactionUsersForPost returns users who reacted to a post.
func GetReactionUsersForPost(postID int, limit int) ([]models.ReactionUserDetail, error) {
	if limit <= 0 {
		limit = 25
	}
	query := `
		SELECT u.id, u.full_name, u.username, COALESCE(u.profile_photo, ''), r.reaction
		FROM reactions r
		JOIN users u ON r.user_id = u.id
		WHERE r.post_id = ?
		ORDER BY r.created_at DESC
		LIMIT ?
	`
	rows, err := database.DB.Query(query, postID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.ReactionUserDetail
	for rows.Next() {
		var u models.ReactionUserDetail
		var photo string
		if err := rows.Scan(&u.UserID, &u.FullName, &u.Username, &photo, &u.ReactionType); err == nil {
			u.ProfilePhoto = photo
			u.Emoji = models.GetReactionEmoji(u.ReactionType)
			users = append(users, u)
		}
	}
	return users, nil
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

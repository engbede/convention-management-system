package services

import (
	"database/sql"
	"errors"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
)

// ReactToPost adds, updates, or toggles off a reaction on a post.
func ReactToPost(
	postID int,
	userID int,
	reaction string,
) error {
	if postID <= 0 || userID <= 0 {
		return errors.New("invalid post or user ID")
	}

	reaction = strings.TrimSpace(reaction)
	if reaction == "" {
		return errors.New("reaction type cannot be empty")
	}

	if !models.IsValidReactionType(reaction) {
		return errors.New("unsupported reaction type")
	}

	currentReaction, err := repository.GetReaction(
		postID,
		userID,
	)

	if err == sql.ErrNoRows {
		return repository.AddReaction(
			postID,
			userID,
			reaction,
		)
	}

	if err != nil {
		return err
	}

	if currentReaction == reaction {
		// Toggle off if clicking the same reaction
		return repository.RemoveReaction(
			postID,
			userID,
		)
	}

	return repository.UpdateReaction(
		postID,
		userID,
		reaction,
	)
}

// PostReactionStats models the response data for post reaction actions and popups.
type PostReactionStats struct {
	PostID            int                        `json:"post_id"`
	UserReaction      string                     `json:"user_reaction"`
	UserReactionEmoji string                     `json:"user_reaction_emoji"`
	UserReactionLabel string                     `json:"user_reaction_label"`
	TotalReactions    int                        `json:"total_reactions"`
	ReactionCounts    map[string]int             `json:"reaction_counts"`
	TopReactions      []models.ReactionCountItem `json:"top_reactions"`
	ReactedUsers      []models.ReactionUserDetail `json:"reacted_users,omitempty"`
}

// GetPostReactionData retrieves complete reaction stats for a post.
func GetPostReactionData(postID, currentUserID int) (*PostReactionStats, error) {
	counts, total, err := repository.GetPostReactionCounts(postID)
	if err != nil {
		return nil, err
	}

	top, _ := repository.GetTopReactionCounts(postID, 5)

	var userReaction string
	if currentUserID > 0 {
		userReaction, _ = repository.GetReaction(postID, currentUserID)
	}

	users, _ := repository.GetReactionUsersForPost(postID, 25)

	stats := &PostReactionStats{
		PostID:            postID,
		UserReaction:      userReaction,
		UserReactionEmoji: models.GetReactionEmoji(userReaction),
		UserReactionLabel: models.GetReactionLabel(userReaction),
		TotalReactions:    total,
		ReactionCounts:    counts,
		TopReactions:      top,
		ReactedUsers:      users,
	}

	return stats, nil
}

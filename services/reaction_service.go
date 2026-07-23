package services

import (
	"database/sql"

	"convention-management-system/repository"
)

func ReactToPost(
	postID int,
	userID int,
	reaction string,
) error {

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
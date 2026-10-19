package repository

import (
	"database/sql"
	"fmt"
	"time"

	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
)

// SavePushSubscription creates or updates a Web Push subscription for a user.
func SavePushSubscription(sub *models.PushSubscription) error {
	var existingID int
	queryCheck := fmt.Sprintf(`SELECT id FROM push_subscriptions WHERE endpoint = %s`, helpers.Placeholder(1))
	err := database.DB.QueryRow(queryCheck, sub.Endpoint).Scan(&existingID)

	if err == sql.ErrNoRows {
		queryInsert := fmt.Sprintf(`
			INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, created_at, updated_at)
			VALUES (%s, %s, %s, %s, %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))
		res, iErr := database.DB.Exec(queryInsert, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth, sub.UserAgent)
		if iErr != nil {
			return iErr
		}
		if id, lErr := res.LastInsertId(); lErr == nil && id > 0 {
			sub.ID = int(id)
		}
		return nil
	} else if err != nil {
		return err
	}

	// Update existing subscription
	queryUpdate := fmt.Sprintf(`
		UPDATE push_subscriptions
		SET user_id = %s, p256dh = %s, auth = %s, user_agent = %s, updated_at = CURRENT_TIMESTAMP
		WHERE endpoint = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))
	_, uErr := database.DB.Exec(queryUpdate, sub.UserID, sub.P256dh, sub.Auth, sub.UserAgent, sub.Endpoint)
	sub.ID = existingID
	return uErr
}

// GetPushSubscriptionsByUserID retrieves all active push subscriptions registered for a given user.
func GetPushSubscriptionsByUserID(userID int) ([]models.PushSubscription, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, endpoint, p256dh, auth, COALESCE(user_agent, ''), created_at, updated_at
		FROM push_subscriptions
		WHERE user_id = %s
		ORDER BY updated_at DESC
	`, helpers.Placeholder(1))

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.PushSubscription
	for rows.Next() {
		var s models.PushSubscription
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.Endpoint,
			&s.P256dh,
			&s.Auth,
			&s.UserAgent,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		s.CreatedAt = createdAt
		s.UpdatedAt = updatedAt
		subs = append(subs, s)
	}

	return subs, nil
}

// DeletePushSubscriptionByEndpoint removes an invalid, expired, or unregistered push subscription.
func DeletePushSubscriptionByEndpoint(endpoint string) error {
	query := fmt.Sprintf(`DELETE FROM push_subscriptions WHERE endpoint = %s`, helpers.Placeholder(1))
	_, err := database.DB.Exec(query, endpoint)
	return err
}

// DeletePushSubscriptionByUserAndEndpoint removes a push subscription matching a user and endpoint.
func DeletePushSubscriptionByUserAndEndpoint(userID int, endpoint string) error {
	query := fmt.Sprintf(`DELETE FROM push_subscriptions WHERE user_id = %s AND endpoint = %s`, helpers.Placeholder(1), helpers.Placeholder(2))
	_, err := database.DB.Exec(query, userID, endpoint)
	return err
}


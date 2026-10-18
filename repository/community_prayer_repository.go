package repository

import (
	"database/sql"
	"fmt"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
)

// CreateCommunityPrayerRequest inserts a new prayer request into the database
func CreateCommunityPrayerRequest(req *models.CommunityPrayerRequest) error {
	if req.PrivacyScope == "" {
		req.PrivacyScope = "public"
	}
	if req.Status == "" {
		req.Status = "active"
	}

	query := `
		INSERT INTO community_prayer_requests (
			user_id,
			title,
			description,
			privacy_scope,
			pray_count,
			status,
			created_at
		) VALUES ($1, $2, $3, $4, 0, $5, $6)
	`
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}

	res, err := database.DB.Exec(query, req.UserID, req.Title, req.Description, req.PrivacyScope, req.Status, req.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed creating community prayer request: %w", err)
	}

	if id, err := res.LastInsertId(); err == nil && id > 0 {
		req.ID = int(id)
	}
	return nil
}

// GetCommunityPrayerRequests fetches visible prayer requests for the given viewer
func GetCommunityPrayerRequests(viewerUserID int, isStaff bool, limit int) ([]models.CommunityPrayerRequest, error) {
	if limit <= 0 {
		limit = 10
	}

	var scopeCondition string
	if viewerUserID > 0 {
		if isStaff {
			scopeCondition = `AND (r.privacy_scope IN ('public', 'fellowship', 'pastoral') OR r.user_id = $2)`
		} else {
			scopeCondition = `AND (r.privacy_scope IN ('public', 'fellowship') OR r.user_id = $2)`
		}
	} else {
		scopeCondition = `AND r.privacy_scope = 'public'`
	}

	query := fmt.Sprintf(`
		SELECT
			r.id,
			r.user_id,
			u.full_name,
			u.username,
			COALESCE(u.profile_photo, ''),
			r.title,
			r.description,
			r.privacy_scope,
			r.pray_count,
			r.status,
			r.created_at,
			CASE WHEN prp.id IS NOT NULL THEN 1 ELSE 0 END AS has_prayed
		FROM community_prayer_requests r
		JOIN users u ON u.id = r.user_id
		LEFT JOIN prayer_request_prayers prp ON prp.prayer_request_id = r.id AND prp.user_id = $1
		WHERE r.status = 'active'
		%s
		ORDER BY r.created_at DESC
		LIMIT %d
	`, scopeCondition, limit)

	var rows *sql.Rows
	var err error
	if viewerUserID > 0 {
		rows, err = database.DB.Query(query, viewerUserID, viewerUserID)
	} else {
		rows, err = database.DB.Query(query, 0)
	}

	if err != nil {
		return nil, fmt.Errorf("failed querying community prayer requests: %w", err)
	}
	defer rows.Close()

	var requests []models.CommunityPrayerRequest
	for rows.Next() {
		var req models.CommunityPrayerRequest
		var hasPrayedInt int
		err := rows.Scan(
			&req.ID,
			&req.UserID,
			&req.AuthorName,
			&req.AuthorUsername,
			&req.AuthorAvatar,
			&req.Title,
			&req.Description,
			&req.PrivacyScope,
			&req.PrayCount,
			&req.Status,
			&req.CreatedAt,
			&hasPrayedInt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning prayer request row: %w", err)
		}
		req.HasPrayed = (hasPrayedInt == 1)
		req.IsOwner = (viewerUserID > 0 && req.UserID == viewerUserID)
		requests = append(requests, req)
	}

	return requests, nil
}

// TogglePrayerRequestPray atomically toggles prayer intercession for a user
func TogglePrayerRequestPray(requestID, userID int) (bool, int, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return false, 0, fmt.Errorf("failed beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if already prayed
	var existingID int
	checkErr := tx.QueryRow(`
		SELECT id FROM prayer_request_prayers
		WHERE prayer_request_id = $1 AND user_id = $2
	`, requestID, userID).Scan(&existingID)

	hasPrayed := false
	if checkErr == nil {
		// Already prayed: remove prayer
		_, err := tx.Exec(`
			DELETE FROM prayer_request_prayers
			WHERE prayer_request_id = $1 AND user_id = $2
		`, requestID, userID)
		if err != nil {
			return false, 0, fmt.Errorf("failed removing prayer intercession: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE community_prayer_requests
			SET pray_count = CASE WHEN pray_count > 0 THEN pray_count - 1 ELSE 0 END
			WHERE id = $1
		`, requestID)
		if err != nil {
			return false, 0, fmt.Errorf("failed decrementing pray count: %w", err)
		}
		hasPrayed = false
	} else if checkErr == sql.ErrNoRows {
		// Not yet prayed: insert prayer
		_, err := tx.Exec(`
			INSERT INTO prayer_request_prayers (prayer_request_id, user_id, created_at)
			VALUES ($1, $2, $3)
		`, requestID, userID, time.Now())
		if err != nil {
			return false, 0, fmt.Errorf("failed recording prayer intercession: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE community_prayer_requests
			SET pray_count = pray_count + 1
			WHERE id = $1
		`, requestID)
		if err != nil {
			return false, 0, fmt.Errorf("failed incrementing pray count: %w", err)
		}
		hasPrayed = true
	} else {
		return false, 0, fmt.Errorf("failed checking prayer intercession: %w", checkErr)
	}

	// Fetch updated count
	var updatedCount int
	err = tx.QueryRow(`SELECT pray_count FROM community_prayer_requests WHERE id = $1`, requestID).Scan(&updatedCount)
	if err != nil {
		return false, 0, fmt.Errorf("failed fetching updated pray count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, 0, fmt.Errorf("failed committing prayer transaction: %w", err)
	}

	return hasPrayed, updatedCount, nil
}

// GetCommunityStats returns aggregate activity metrics
func GetCommunityStats() (*models.CommunityStats, error) {
	stats := &models.CommunityStats{}

	_ = database.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.MembersCount)
	_ = database.DB.QueryRow("SELECT COUNT(*) FROM posts").Scan(&stats.PostsCount)
	_ = database.DB.QueryRow("SELECT COUNT(*) FROM reactions").Scan(&stats.ReactionsCount)
	_ = database.DB.QueryRow("SELECT COUNT(*) FROM comments").Scan(&stats.CommentsCount)

	return stats, nil
}


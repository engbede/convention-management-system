package repository

import (
	"database/sql"
	"fmt"

	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
)

// HasUserAcceptedTerms checks if the user has accepted a specific version of platform terms.
func HasUserAcceptedTerms(userID int, version string) (bool, error) {
	if database.DB == nil {
		return false, fmt.Errorf("database connection not initialized")
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM terms_acceptances
		WHERE user_id = %s AND terms_version = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var count int
	err := database.DB.QueryRow(query, userID, version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed checking terms acceptance: %w", err)
	}

	return count > 0, nil
}

// RecordTermsAcceptance stores an audit record of terms acceptance.
func RecordTermsAcceptance(userID int, version, ip, userAgent string) error {
	if database.DB == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := fmt.Sprintf(`
		INSERT INTO terms_acceptances (user_id, terms_version, ip_address, user_agent, accepted_at)
		VALUES (%s, %s, %s, %s, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, terms_version)
		DO UPDATE SET ip_address = EXCLUDED.ip_address, user_agent = EXCLUDED.user_agent, accepted_at = CURRENT_TIMESTAMP
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))

	_, err := database.DB.Exec(query, userID, version, ip, userAgent)
	if err != nil {
		return fmt.Errorf("failed recording terms acceptance: %w", err)
	}

	return nil
}

// GetLatestTermsAcceptance retrieves the most recent terms acceptance for a user.
func GetLatestTermsAcceptance(userID int) (*models.TermsAcceptance, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, terms_version, accepted_at, ip_address, user_agent
		FROM terms_acceptances
		WHERE user_id = %s
		ORDER BY accepted_at DESC, id DESC
		LIMIT 1
	`, helpers.Placeholder(1))

	var t models.TermsAcceptance
	var rawAcceptedAt any
	err := database.DB.QueryRow(query, userID).Scan(
		&t.ID,
		&t.UserID,
		&t.TermsVersion,
		&rawAcceptedAt,
		&t.IPAddress,
		&t.UserAgent,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed fetching latest terms acceptance: %w", err)
	}

	t.AcceptedAt = parseSQLTime(rawAcceptedAt)
	return &t, nil
}

// GetTermsAcceptanceHistory returns all terms acceptance audit records for a user.
func GetTermsAcceptanceHistory(userID int) ([]models.TermsAcceptance, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, terms_version, accepted_at, ip_address, user_agent
		FROM terms_acceptances
		WHERE user_id = %s
		ORDER BY accepted_at DESC, id DESC
	`, helpers.Placeholder(1))

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed querying terms acceptance history: %w", err)
	}
	defer rows.Close()

	var history []models.TermsAcceptance
	for rows.Next() {
		var t models.TermsAcceptance
		var rawAcceptedAt any
		if err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.TermsVersion,
			&rawAcceptedAt,
			&t.IPAddress,
			&t.UserAgent,
		); err != nil {
			return nil, fmt.Errorf("failed scanning terms acceptance: %w", err)
		}
		t.AcceptedAt = parseSQLTime(rawAcceptedAt)
		history = append(history, t)
	}

	return history, nil
}


package repository

import (
	"database/sql"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
)

func CreatePasswordReset(reset *models.PasswordReset) error {
	query := `
	INSERT INTO password_resets (user_id, email, token_hash, expires_at, used, created_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	`
	if reset.CreatedAt.IsZero() {
		reset.CreatedAt = time.Now()
	}
	res, err := database.DB.Exec(
		query,
		reset.UserID,
		reset.Email,
		reset.TokenHash,
		reset.ExpiresAt,
		reset.Used,
		reset.CreatedAt,
	)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		reset.ID = int(id)
	} else {
		_ = database.DB.QueryRow("SELECT id FROM password_resets WHERE token_hash = $1", reset.TokenHash).Scan(&reset.ID)
	}
	return nil
}

func GetValidPasswordResetByTokenHash(tokenHash string) (*models.PasswordReset, error) {
	query := `
	SELECT id, user_id, email, token_hash, expires_at, used, created_at
	FROM password_resets
	WHERE token_hash = $1 AND used = 0 AND expires_at > $2
	ORDER BY id DESC
	LIMIT 1
	`
	var reset models.PasswordReset
	err := database.DB.QueryRow(query, tokenHash, time.Now()).Scan(
		&reset.ID,
		&reset.UserID,
		&reset.Email,
		&reset.TokenHash,
		&reset.ExpiresAt,
		&reset.Used,
		&reset.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &reset, nil
}

func UpdateUserPasswordAndInvalidateReset(userID int, newPasswordHash string, tokenHash string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Verify token is still unused and not expired in transaction
	var resetID int
	var used int
	var expiresAt time.Time
	err = tx.QueryRow(`
		SELECT id, used, expires_at 
		FROM password_resets 
		WHERE token_hash = $1
	`, tokenHash).Scan(&resetID, &used, &expiresAt)
	if err != nil {
		return err
	}
	if used == 1 {
		return sql.ErrNoRows
	}
	if time.Now().After(expiresAt) {
		return sql.ErrNoRows
	}

	// 2. Mark token used
	_, err = tx.Exec(`
		UPDATE password_resets 
		SET used = 1 
		WHERE id = $1
	`, resetID)
	if err != nil {
		return err
	}

	// 3. Update user password
	_, err = tx.Exec(`
		UPDATE users
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`, newPasswordHash, time.Now(), userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func UpdateAdminPasswordAndInvalidateReset(adminID int, newPasswordHash string, tokenHash string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Verify token is still unused and not expired in transaction
	var resetID int
	var used int
	var expiresAt time.Time
	err = tx.QueryRow(`
		SELECT id, used, expires_at 
		FROM password_resets 
		WHERE token_hash = $1
	`, tokenHash).Scan(&resetID, &used, &expiresAt)
	if err != nil {
		return err
	}
	if used == 1 || time.Now().After(expiresAt) {
		return sql.ErrNoRows
	}

	// 2. Mark token used
	_, err = tx.Exec(`
		UPDATE password_resets 
		SET used = 1 
		WHERE id = $1
	`, resetID)
	if err != nil {
		return err
	}

	// 3. Update admin password
	_, err = tx.Exec(`
		UPDATE admins
		SET password = $1
		WHERE id = $2
	`, newPasswordHash, adminID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func CountRecentResetsByEmail(email string, window time.Duration) (int, error) {
	since := time.Now().Add(-window)
	query := `
	SELECT COUNT(*)
	FROM password_resets
	WHERE LOWER(email) = LOWER($1) AND created_at >= $2
	`
	var count int
	err := database.DB.QueryRow(query, email, since).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}


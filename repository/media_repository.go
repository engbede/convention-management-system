package repository

import (
	"database/sql"
	"strings"

	"convention-management-system/database"
)

// SaveMediaBlob durably persists media bytes into PostgreSQL or SQLite.
// The key is normalized (stripping any leading slashes or "uploads/").
func SaveMediaBlob(key, contentType string, data []byte) error {
	if database.DB == nil || len(data) == 0 {
		return nil
	}

	key = strings.TrimPrefix(key, "/")
	key = strings.TrimPrefix(key, "uploads/")
	key = strings.TrimPrefix(key, "/")

	_, err := database.DB.Exec(`
		INSERT INTO media_storage(media_key, content_type, file_size, data)
		VALUES($1, $2, $3, $4)
		ON CONFLICT(media_key)
		DO UPDATE SET
			content_type = EXCLUDED.content_type,
			file_size = EXCLUDED.file_size,
			data = EXCLUDED.data,
			created_at = CURRENT_TIMESTAMP
	`, key, contentType, int64(len(data)), data)

	return err
}

// GetMediaBlob retrieves media bytes and content type by key.
// It checks the exact key and folder prefixes (posts/, profiles/, covers/, chat/).
func GetMediaBlob(key string) ([]byte, string, error) {
	if database.DB == nil {
		return nil, "", sql.ErrNoRows
	}

	key = strings.TrimPrefix(key, "/")
	key = strings.TrimPrefix(key, "uploads/")
	key = strings.TrimPrefix(key, "/")

	var contentType string
	var data []byte

	// 1. Try exact match
	err := database.DB.QueryRow(`
		SELECT content_type, data
		FROM media_storage
		WHERE media_key = $1
	`, key).Scan(&contentType, &data)

	if err == nil {
		return data, contentType, nil
	}

	// 2. If key doesn't have a known prefix, try each prefix
	prefixes := []string{"posts/", "profiles/", "covers/", "chat/"}
	for _, p := range prefixes {
		if !strings.HasPrefix(key, p) {
			candidate := p + key
			if qErr := database.DB.QueryRow(`
				SELECT content_type, data
				FROM media_storage
				WHERE media_key = $1
			`, candidate).Scan(&contentType, &data); qErr == nil {
				return data, contentType, nil
			}
		}
	}

	// 3. If key starts with a known prefix, try stripping it
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			candidate := strings.TrimPrefix(key, p)
			if qErr := database.DB.QueryRow(`
				SELECT content_type, data
				FROM media_storage
				WHERE media_key = $1
			`, candidate).Scan(&contentType, &data); qErr == nil {
				return data, contentType, nil
			}
		}
	}

	return nil, "", err
}

// HasMediaBlob checks if a media key exists in durable storage without reading the full blob.
func HasMediaBlob(key string) bool {
	if database.DB == nil {
		return false
	}
	key = strings.TrimPrefix(key, "/")
	key = strings.TrimPrefix(key, "uploads/")
	key = strings.TrimPrefix(key, "/")

	var exists int
	err := database.DB.QueryRow(`
		SELECT 1 FROM media_storage WHERE media_key = $1
	`, key).Scan(&exists)
	if err == nil {
		return true
	}

	prefixes := []string{"posts/", "profiles/", "covers/", "chat/"}
	for _, p := range prefixes {
		if !strings.HasPrefix(key, p) {
			if qErr := database.DB.QueryRow(`
				SELECT 1 FROM media_storage WHERE media_key = $1
			`, p+key).Scan(&exists); qErr == nil {
				return true
			}
		}
	}
	return false
}

// DeleteMediaBlob removes media by key and common prefixed aliases.
func DeleteMediaBlob(key string) error {
	if database.DB == nil {
		return nil
	}

	key = strings.TrimPrefix(key, "/")
	key = strings.TrimPrefix(key, "uploads/")
	key = strings.TrimPrefix(key, "/")

	_, err := database.DB.Exec(`
		DELETE FROM media_storage
		WHERE media_key = $1 OR media_key = $2 OR media_key = $3
	`, key, "posts/"+key, "profiles/"+key)

	return err
}

// MediaStorageItem represents a metadata entry without the full blob data.
type MediaStorageItem struct {
	MediaKey    string
	ContentType string
	FileSize    int64
}

// ListMediaStorageKeys returns all media keys present in durable storage.
func ListMediaStorageKeys() ([]MediaStorageItem, error) {
	if database.DB == nil {
		return nil, nil
	}

	rows, err := database.DB.Query(`
		SELECT media_key, content_type, file_size
		FROM media_storage
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MediaStorageItem
	for rows.Next() {
		var item MediaStorageItem
		if err := rows.Scan(&item.MediaKey, &item.ContentType, &item.FileSize); err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}


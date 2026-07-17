package repository

import "convention-management-system/database"

func GetSetting(key string) string {

	var value string

	_ = database.DB.QueryRow(
		"SELECT value FROM settings WHERE key = $1",
		key,
	).Scan(&value)

	return value
}

func SaveSetting(
	key string,
	value string,
) error {

	_, err := database.DB.Exec(`
INSERT INTO settings(key, value)
VALUES($1, $2)
ON CONFLICT(key)
DO UPDATE SET value = EXCLUDED.value
`,
		key,
		value,
	)

	return err
}

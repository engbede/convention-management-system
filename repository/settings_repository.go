package repository

import "convention-management-system/database"

func GetSetting(key string) string {

	var value string

	_ = database.DB.QueryRow(
		"SELECT value FROM settings WHERE key=?",
		key,
	).Scan(&value)

	return value
}

func SaveSetting(
	key string,
	value string,
) error {

	_, err := database.DB.Exec(`
INSERT INTO settings(key,value)
VALUES(?,?)
ON CONFLICT(key)
DO UPDATE SET value=excluded.value
`,
		key,
		value,
	)

	return err
}

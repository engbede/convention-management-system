package repository

import "convention-management-system/database"

type AttendanceStats struct {
	CheckedIn int
	Pending   int
}

func GetAttendanceStats() (AttendanceStats, error) {

	var stats AttendanceStats

	err := database.DB.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN checked_in = TRUE THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN checked_in = FALSE THEN 1 ELSE 0 END), 0)
		FROM registrations
	`).Scan(
		&stats.CheckedIn,
		&stats.Pending,
	)

	return stats, err
}

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
			SUM(CASE WHEN checked_in = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN checked_in = 0 THEN 1 ELSE 0 END)
		FROM registrations
	`).Scan(
		&stats.CheckedIn,
		&stats.Pending,
	)

	return stats, err
}

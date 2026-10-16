package repository

import "convention-management-system/database"

type ReportStats struct {
	TotalRegistrations int
	TotalOfficials     int
	TotalConventions   int
	CheckedIn          int
}

func GetReportStats() (ReportStats, error) {

	var stats ReportStats

	err := database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM registrations
	`).Scan(&stats.TotalRegistrations)

	if err != nil {
		return stats, err
	}

	err = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM officials
	`).Scan(&stats.TotalOfficials)

	if err != nil {
		return stats, err
	}

	err = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM conventions
	`).Scan(&stats.TotalConventions)

	if err != nil {
		return stats, err
	}

	err = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM registrations
		WHERE checked_in = TRUE
	`).Scan(&stats.CheckedIn)

	if err != nil {
		return stats, err
	}

	return stats, nil
}

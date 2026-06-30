package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

func Dashboard(
	w http.ResponseWriter,
	r *http.Request,
) {

	stats, err := repository.GetDashboardStats()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
	attendance, err := repository.GetAttendanceStats()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
	circuits, err := repository.GetRegistrationsByCircuit()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	stats.Circuits = circuits

	stats.CheckedIn = attendance.CheckedIn

	stats.Pending = attendance.Pending

	if stats.TotalRegistrations > 0 {

		stats.AttendanceRate =
			float64(attendance.CheckedIn) /
				float64(stats.TotalRegistrations) * 100
	}
	err = Templates.ExecuteTemplate(
		w,
		"dashboard.html",
		stats,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}

}

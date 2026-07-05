package handlers

import (
	"log"
	"net/http"

	"convention-management-system/repository"
)

func Dashboard(
	w http.ResponseWriter,
	r *http.Request,
) {

	stats, err := repository.GetDashboardStats()
	if err != nil {
		log.Println("DashboardStats:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	activeConvention, err := repository.GetActiveConvention()
	if err != nil {
		log.Println("Active Convention:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	circuits, err := repository.GetRegistrationsByCircuit()
	if err != nil {
		log.Println("Circuits:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	attendance, err := repository.GetAttendanceStats()
	if err != nil {
		log.Println("Attendance:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	data := struct {
		Title      string
		Stats      interface{}
		Convention interface{}
	}{
		Title:      "Administrator Dashboard",
		Stats:      stats,
		Convention: activeConvention,
	}

	err = Templates.ExecuteTemplate(
		w,
		"admin_layout",
		data,
	)
	
	if err != nil {
		log.Println("Dashboard Template Error:", err)
		return
	}
}

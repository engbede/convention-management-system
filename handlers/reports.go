package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

func Reports(
	w http.ResponseWriter,
	r *http.Request,
) {

	stats, err := repository.GetReportStats()

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	data := struct {
		Title string
		Stats repository.ReportStats
	}{
		Title: "Reports & Analytics",
		Stats: stats,
	}

	Render(
		w,
		"reports.html",
		data,
	)
}

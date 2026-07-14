package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

func Home(
	w http.ResponseWriter,
	r *http.Request,
) {

	convention, err := repository.GetActiveConvention()

	if err != nil {

		http.Error(
			w,
			"No active convention found.",
			http.StatusInternalServerError,
		)

		return
	}

	notices, _ := repository.GetActiveNotices()

	data := struct {
		Convention interface{}
		Notices    interface{}
	}{
		Convention: convention,
		Notices:    notices,
	}

	Render(
		w,
		"home.html",
		data,
	)
}

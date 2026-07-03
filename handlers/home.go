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

	err = Templates.ExecuteTemplate(
		w,
		"home.html",
		convention,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
}

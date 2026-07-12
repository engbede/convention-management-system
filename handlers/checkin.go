package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

func QRCheckIn(
	w http.ResponseWriter,
	r *http.Request,
) {

	number := r.URL.Query().Get("reg")

	if number == "" {

		http.Error(
			w,
			"Missing registration number.",
			http.StatusBadRequest,
		)

		return
	}

	reg, err := repository.GetRegistrationByNumber(number)

	if err != nil {

		Render(
			w,
			"checkin_error.html",
			"Registration not found.",
		)

		return
	}

	if reg.CheckedIn {

		Render(
			w,
			"checkin_error.html",
			"Attendee has already checked in.",
		)

		return
	}
	err = repository.MarkCheckedIn(reg.ID)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	Render(
		w,
		"checkin_success.html",
		reg,
	)
}

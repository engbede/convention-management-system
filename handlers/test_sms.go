package handlers

import (
	"net/http"

	"convention-management-system/services"
)

func TestSMS(
	w http.ResponseWriter,
	r *http.Request,
) {

	err := services.SendSMS(
		"2347053114787",
		"Hello Emmanuel! Your Infobip integration is working.",
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	w.Write([]byte("SMS sent successfully"))
}

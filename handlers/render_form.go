package handlers

import (
	"net/http"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func renderRegistrationForm(
	w http.ResponseWriter,
	reg models.Registration,
	errMsg string,
) {

	convention, _ := repository.GetActiveConvention()

	notices, _ := repository.GetActiveNotices()

	data := models.FormData{
		Title:        "Youth Convention Registration",
		Action:       "/submit-registration",
		ButtonText:   "Register",
		Convention:   convention,
		Registration: reg,
		Notices:      notices,
		Error:        errMsg,
	}

	Render(
		w,
		"form.html",
		data,
	)
}

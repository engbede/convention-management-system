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

	data := models.FormData{
		Title:        "Youth Convention Registration",
		Action:       "/submit-registration",
		ButtonText:   "Register",
		Convention:   convention,
		Registration: reg,
		Error:        errMsg,
	}

	err := Templates.ExecuteTemplate(
		w,
		"form.html",
		data,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}
}

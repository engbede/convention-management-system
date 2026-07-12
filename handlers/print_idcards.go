package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/helpers"
	"convention-management-system/repository"
)

func PrintIDCards(
	w http.ResponseWriter,
	r *http.Request,
) {

	registrations, err := repository.GetAllRegistrations()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	for i := range registrations {

		qrText := `METHODIST CHURCH NIGERIA
DIOCESE OF APA

ANNUAL YOUTH CONVENTION

Registration Number: ` + registrations[i].RegistrationNumber + `

Name: ` + registrations[i].FullName + `

Gender: ` + registrations[i].Gender + `

Age: ` + strconv.Itoa(registrations[i].Age) + `

Phone: ` + registrations[i].Phone + `

Circuit: ` + registrations[i].Circuit + `

Church: ` + registrations[i].LocalChurch + `

Bible Study Group: Group ` + strconv.Itoa(registrations[i].BibleStudyGroup)

		qr, err := helpers.GenerateQRCode(qrText)

		if err == nil {
			registrations[i].QRCode = qr
		}
	}

	data := struct {
		Title         string
		Registrations interface{}
	}{
		Title:         "Print ID Cards",
		Registrations: registrations,
	}

	Render(
		w,
		"print_idcards.html",
		registrations,
	)
}

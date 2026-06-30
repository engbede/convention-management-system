package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/helpers"
	"convention-management-system/repository"
)

func IDCard(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid registration ID",
			http.StatusBadRequest,
		)

		return
	}

	reg, err := repository.GetRegistrationByID(id)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	qrText := `METHODIST CHURCH NIGERIA
DIOCESE OF APA

ANNUAL YOUTH CONVENTION

Registration Number: ` + reg.RegistrationNumber + `

Name: ` + reg.FullName + `

Gender: ` + reg.Gender + `

Age: ` + strconv.Itoa(reg.Age) + `

Phone: ` + reg.Phone + `

Circuit: ` + reg.Circuit + `

Church: ` + reg.LocalChurch + `

Bible Study Group: Group ` + strconv.Itoa(reg.BibleStudyGroup)

	qr, err := helpers.GenerateQRCode(qrText)

	if err == nil {
		reg.QRCode = qr
	}

	err = Templates.ExecuteTemplate(
		w,
		"idcard.html",
		reg,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}
}

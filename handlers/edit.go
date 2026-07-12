package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func EditRegistration(
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

	data := models.FormData{

		Title: "Edit Registration",

		Action: "/update",

		ButtonText: "Update Registration",

		Registration: reg,
	}

	Render(
		w,
		"form.html",
		data,
	)
}

func UpdateRegistration(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/registrations",
			http.StatusSeeOther,
		)

		return
	}

	id, err := strconv.Atoi(
		r.FormValue("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid registration ID",
			http.StatusBadRequest,
		)

		return
	}

	age, err := strconv.Atoi(
		r.FormValue("age"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid age",
			http.StatusBadRequest,
		)

		return
	}

	group, err := strconv.Atoi(r.FormValue("bible_study_group"))
	if err != nil {
		http.Error(w, "Invalid Bible Study Group", http.StatusBadRequest)
		return
	}

	firstTime :=
		r.FormValue("first_time_attendee") == "Yes"

	reg := models.Registration{

		ID: id,

		FullName: r.FormValue("fullname"),

		Gender: r.FormValue("gender"),

		Age: age,

		Phone: r.FormValue("phone"),

		Circuit: r.FormValue("circuit"),

		LocalChurch: r.FormValue("local_church"),

		Membership: r.FormValue("membership"),

		Position: r.FormValue("position"),

		MaritalStatus: r.FormValue("marital_status"),

		Occupation: r.FormValue("occupation"),

		EmergencyContactName: r.FormValue("emergency_contact_name"),

		EmergencyContactPhone: r.FormValue("emergency_contact_phone"),

		ArrivalDate: r.FormValue("arrival_date"),

		FirstTimeAttendee: firstTime,

		BibleStudyGroup: group,
	}

	err = repository.UpdateRegistration(reg)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	http.Redirect(
		w,
		r,
		"/registrations",
		http.StatusSeeOther,
	)
}

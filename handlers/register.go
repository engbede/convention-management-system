package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

var Templates *template.Template

// Display registration form
func ShowForm(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := models.FormData{
		Title:      "Youth Convention Registration",
		Action:     "/register",
		ButtonText: "Register",
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

// Handle registration
func Register(
	w http.ResponseWriter,
	r *http.Request,
) {

	// Only allow POST
	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/",
			http.StatusSeeOther,
		)

		return
	}

	// Convert age to integer
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

	// Convert Yes/No to bool
	firstTime :=
		r.FormValue("first_time_attendee") == "Yes"

	group, err := strconv.Atoi(
		r.FormValue("bible_study_group"),
	)

	if err != nil {
		group = 0
	}

	// Create Registration object
	reg := models.Registration{

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

		BibleStudyGroup: group,

		FirstTimeAttendee: firstTime,
	}

	// Save into SQLite
	err = repository.CreateRegistration(reg)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// Show success page
	err = Templates.ExecuteTemplate(
		w,
		"success.html",
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

func ListRegistrations(
	w http.ResponseWriter,
	r *http.Request,
) {

	search := r.URL.Query().Get("search")

	page := 1
	pageSize := 10

	if p := r.URL.Query().Get("page"); p != "" {

		value, err := strconv.Atoi(p)

		if err == nil && value > 0 {
			page = value
		}
	}

	var (
		registrations []models.Registration
		total         int
		err           error
	)

	if search == "" {

		registrations, err =
			repository.GetRegistrationsPaginated(
				page,
				pageSize,
			)

		if err != nil {

			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)

			return
		}

		total, err =
			repository.CountRegistrations()

		if err != nil {

			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)

			return
		}

	} else {

		registrations, err =
			repository.SearchRegistrations(search)

		if err != nil {

			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)

			return
		}

		total = len(registrations)
	}

	totalPages := (total + pageSize - 1) / pageSize

	data := struct {
		Registrations []models.Registration
		Search        string
		Page          int
		TotalPages    int
		HasPrevious   bool
		HasNext       bool
		PreviousPage  int
		NextPage      int
	}{
		Registrations: registrations,
		Search:        search,
		Page:          page,
		TotalPages:    totalPages,
		HasPrevious:   page > 1,
		HasNext:       page < totalPages,
		PreviousPage:  page - 1,
		NextPage:      page + 1,
	}

	err = Templates.ExecuteTemplate(
		w,
		"registrations.html",
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

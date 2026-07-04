package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
)

var Templates *template.Template

// renderRegistrationForm redisplays the registration form with an error message.

// ShowForm displays the registration page.
func ShowForm(
	w http.ResponseWriter,
	r *http.Request,
) {

	activeConvention, err := repository.GetActiveConvention()

	if err != nil {
		http.Error(
			w,
			"No active convention found. Please contact the administrator.",
			http.StatusInternalServerError,
		)
		return
	}

	data := models.FormData{
		Title:      "Youth Convention Registration",
		Action:     "/submit-registration",
		ButtonText: "Register",
		Convention: activeConvention,
	}

	err = Templates.ExecuteTemplate(
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

// Register handles registration submission.
func Register(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/register",
			http.StatusSeeOther,
		)
		return
	}

	fullName := r.FormValue("fullname")
	gender := r.FormValue("gender")
	phone := r.FormValue("phone")
	circuit := r.FormValue("circuit")
	localChurch := r.FormValue("local_church")
	membership := r.FormValue("membership")
	position := r.FormValue("position")
	maritalStatus := r.FormValue("marital_status")
	occupation := r.FormValue("occupation")
	emergencyName := r.FormValue("emergency_contact_name")
	emergencyPhone := r.FormValue("emergency_contact_phone")
	arrivalDate := r.FormValue("arrival_date")
	// Convert age
	age, err := strconv.Atoi(r.FormValue("age"))
	if err != nil {
		age = 0
	}

	// Convert Bible Study Group
	group, err := strconv.Atoi(r.FormValue("bible_study_group"))
	if err != nil {
		group = 0
	}

	// First-time attendee
	firstTime := r.FormValue("first_time_attendee") == "Yes"

	// Temporary registration object for redisplaying the form
	reg := models.Registration{
		FullName: fullName,
		Gender:   gender,
		Age:      age,
		Phone:    phone,

		Circuit:     circuit,
		LocalChurch: localChurch,
		Membership:  membership,
		Position:    position,

		MaritalStatus: maritalStatus,
		Occupation:    occupation,

		EmergencyContactName:  emergencyName,
		EmergencyContactPhone: emergencyPhone,

		ArrivalDate: arrivalDate,

		BibleStudyGroup:   group,
		FirstTimeAttendee: firstTime,
	}

	// Required fields
	if fullName == "" ||
		gender == "" ||
		phone == "" ||
		circuit == "" ||
		localChurch == "" ||
		membership == "" ||
		emergencyName == "" ||
		emergencyPhone == "" ||
		arrivalDate == "" {

		renderRegistrationForm(
			w,
			reg,
			"Please complete all required fields.",
		)
		return
	}

	// Phone number validation
	matched, _ := regexp.MatchString(
		`^[0-9]{11}$`,
		phone,
	)

	if !matched {
		renderRegistrationForm(
			w,
			reg,
			"Phone number must contain exactly 11 digits.",
		)
		return
	}

	activeConvention, err := repository.GetActiveConvention()

	if err != nil {
		http.Error(
			w,
			"No active convention found.",
			http.StatusBadRequest,
		)
		return
	}
	reg.ConventionID = activeConvention.ID

	exists, err := repository.PhoneExists(phone)

	if err != nil {
		fmt.Println("PhoneExists error:", err)

		renderRegistrationForm(
			w,
			reg,
			"Unable to validate phone number. Please try again.",
		)
		return
	}

	if exists {
		renderRegistrationForm(
			w,
			reg,
			"This phone number has already been used for registration.",
		)
		return
	}

	err = repository.CreateRegistration(reg)

	if err != nil {

		renderRegistrationForm(
			w,
			reg,
			"Registration could not be completed. Please try again.",
		)

		return
	}

	message := fmt.Sprintf(
		"Dear %s,\n\n"+
			"Your registration for %s has been received successfully.\n\n"+
			"Venue: %s\n"+
			"Arrival Date: %s\n"+
			"Bible Study Group: %d\n\n"+
			"Thank you for registering.\n\n"+
			"Methodist Church Nigeria\n"+
			"Apa Diocesan Youth Fellowship",
		reg.FullName,
		activeConvention.Name,
		activeConvention.Venue,
		reg.ArrivalDate,
		reg.BibleStudyGroup,
	)

	if err := services.SendSMS(reg.Phone, message); err != nil {

		// Don't stop registration because SMS failed.
		fmt.Println("SMS Error:", err)

	}

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

// ListRegistrations displays all registrations with search and pagination.
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

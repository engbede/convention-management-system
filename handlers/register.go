package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
)

// renderRegistrationForm redisplays the registration form with an error message.

// ShowForm displays the registration page.
func ShowForm(
	w http.ResponseWriter,
	r *http.Request,
) {
	if repository.GetSetting("registration_open") == "closed" {

		http.Error(
			w,
			"Registration is currently closed.",
			http.StatusForbidden,
		)

		return
	}

	activeConvention, err := repository.GetActiveConvention()

	if err != nil {
		http.Error(
			w,
			"No active convention found. Please contact the administrator.",
			http.StatusInternalServerError,
		)
		return
	}
	notices, _ := repository.GetActiveNotices()
	data := models.FormData{
		Title:      "Youth Convention Registration",
		Action:     "/submit-registration",
		ButtonText: "Register",
		Convention: activeConvention,
		Notices:    notices,
	}

	Render(
		w,
		"form.html",
		data,
	)
}

// Register handles registration submission.
func Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	if repository.GetSetting("registration_open") == "closed" {

		http.Error(
			w,
			"Registration is currently closed.",
			http.StatusForbidden,
		)

		return
	}
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
	email := r.FormValue("email")

	circuit := r.FormValue("circuit")
	localChurch := r.FormValue("local_church")
	membership := r.FormValue("membership")
	position := r.FormValue("position")

	maritalStatus := r.FormValue("marital_status")
	occupation := r.FormValue("occupation")

	emergencyName := r.FormValue("emergency_contact_name")
	emergencyPhone := r.FormValue("emergency_contact_phone")

	relationship := r.FormValue("relationship")
	address := r.FormValue("address")

	arrivalDate := r.FormValue("arrival_date") // Convert age
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

		Phone: phone,
		Email: email,

		Circuit:     circuit,
		LocalChurch: localChurch,
		Membership:  membership,
		Position:    position,

		MaritalStatus: maritalStatus,
		Occupation:    occupation,

		EmergencyContactName:  emergencyName,
		EmergencyContactPhone: emergencyPhone,

		Relationship: relationship,
		Address:      address,

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
	if email != "" {

		matched, _ := regexp.MatchString(
			`^[^\s@]+@[^\s@]+\.[^\s@]+$`,
			email,
		)

		if !matched {

			renderRegistrationForm(
				w,
				reg,
				"Please enter a valid email address or leave it blank.",
			)

			return
		}
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

	err = repository.CreateRegistration(&reg)

	if err != nil {

		renderRegistrationForm(
			w,
			reg,
			"Registration could not be completed. Please try again.",
		)

		return
	}

	// Generate registration number
	reg.RegistrationNumber = repository.GenerateRegistrationNumber(reg.ID)

	err = repository.SaveRegistrationNumber(
		reg.ID,
		reg.RegistrationNumber,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// Generate QR Code
	qrPath, err := services.GenerateQRCode(
		reg.RegistrationNumber,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	reg.QRCode = qrPath

	err = repository.SaveQRCode(
		reg.ID,
		qrPath,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// Convert phone number to international format for SMS
	smsPhone := reg.Phone

	if strings.HasPrefix(smsPhone, "0") {
		smsPhone = "234" + smsPhone[1:]
	}

	fmt.Println("========== SMS DEBUG ==========")
	fmt.Println("Original Phone :", reg.Phone)
	fmt.Println("Converted Phone:", smsPhone)
	fmt.Println("Recipient Name :", reg.FullName)
	fmt.Println("Registration No:", reg.RegistrationNumber)
	fmt.Println("Convention     :", activeConvention.Name)
	fmt.Println("===============================")

	// Short message for testing
	message := fmt.Sprintf(
		"Dear %s,\n\n"+
			"Your registration for %s has been received successfully.\n\n"+
			"Registration No: %s\n\n"+
			"Venue: %s\n"+
			"Arrival Date: %s\n"+
			"Bible Study Group: %d\n\n"+
			"Please keep your registration number. It will be required during check-in.\n\n"+
			"Thank you.\n\n"+
			"Methodist Church Nigeria\n"+
			"Apa Diocesan Youth Fellowship",
		reg.FullName,
		activeConvention.Name,
		reg.RegistrationNumber,
		activeConvention.Venue,
		reg.ArrivalDate,
		reg.BibleStudyGroup,
	)

	fmt.Println("Message:", message)
	fmt.Println("Preparing to send SMS...")

	err = services.SendSMS(smsPhone, message)

	if err != nil {
		fmt.Println("SMS Error:", err)
	} else {
		fmt.Println("SMS sent successfully from registration.")
	}

	Render(
		w,
		"success.html",
		reg,
	)
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
		Title         string
		Registrations []models.Registration
		Search        string
		Page          int
		TotalPages    int
		HasPrevious   bool
		HasNext       bool
		PreviousPage  int
		NextPage      int
	}{
		Title:         "Registrations",
		Registrations: registrations,
		Search:        search,
		Page:          page,
		TotalPages:    totalPages,
		HasPrevious:   page > 1,
		HasNext:       page < totalPages,
		PreviousPage:  page - 1,
		NextPage:      page + 1,
	}

	Render(
		w,
		"registrations.html",
		data,
	)
}

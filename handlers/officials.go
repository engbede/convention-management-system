package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

// ShowOfficialForm displays the official registration form.
func ShowOfficialForm(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title      string
		Action     string
		ButtonText string
		Official   models.Official
		Error      string
	}{
		Title:      "Register Official",
		Action:     "/officials/create",
		ButtonText: "Register Official",
	}

	Render(
		w,
		"official_form.html",
		data,
	)
}

// CreateOfficial saves a new official.
func CreateOfficial(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/officials/new",
			http.StatusSeeOther,
		)
		return
	}

	official := models.Official{
		FullName:    r.FormValue("full_name"),
		Gender:      r.FormValue("gender"),
		Phone:       r.FormValue("phone"),
		Email:       r.FormValue("email"),
		Circuit:     r.FormValue("circuit"),
		LocalChurch: r.FormValue("local_church"),
		Position:    r.FormValue("position"),
		Department:  r.FormValue("department"),
		Status:      r.FormValue("status"),
	}

	if official.FullName == "" ||
		official.Phone == "" ||
		official.Position == "" {

		http.Error(
			w,
			"Please complete all required fields.",
			http.StatusBadRequest,
		)
		return
	}

	err := repository.CreateOfficial(official)
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
		"/officials",
		http.StatusSeeOther,
	)
}

// ListOfficials displays all officials.
func ListOfficials(
	w http.ResponseWriter,
	r *http.Request,
) {

	officials, err := repository.GetAllOfficials()
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	data := struct {
		Title     string
		Officials interface{}
	}{
		Title:     "Officials",
		Officials: officials,
	}

	Render(
		w,
		"officials.html",
		data,
	)
}
func ViewOfficial(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "Invalid official ID", http.StatusBadRequest)
		return
	}

	official, err := repository.GetOfficialByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	Render(
		w,
		"official_view.html",
		struct {
			Title    string
			Official models.Official
		}{
			Title:    "Official Details",
			Official: official,
		},
	)
}
func EditOfficial(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "Invalid official ID", http.StatusBadRequest)
		return
	}

	official, err := repository.GetOfficialByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title    string
		Action   string
		Official models.Official
	}{
		Title:    "Edit Official",
		Action:   "/officials/update",
		Official: official,
	}

	Render(
		w,
		"official_form.html",
		data,
	)
}
func UpdateOfficial(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/officials", http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	official := models.Official{
		ID:          id,
		FullName:    r.FormValue("full_name"),
		Gender:      r.FormValue("gender"),
		Phone:       r.FormValue("phone"),
		Email:       r.FormValue("email"),
		Circuit:     r.FormValue("circuit"),
		LocalChurch: r.FormValue("local_church"),
		Position:    r.FormValue("position"),
		Department:  r.FormValue("department"),
		Status:      r.FormValue("status"),
		Photo:       r.FormValue("photo"),
	}

	err = repository.UpdateOfficial(official)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(
		w,
		r,
		"/officials",
		http.StatusSeeOther,
	)
}
func DeleteOfficial(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = repository.DeleteOfficial(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(
		w,
		r,
		"/officials",
		http.StatusSeeOther,
	)
}

package handlers

import (
	"convention-management-system/models"
	"convention-management-system/repository"
	"fmt"
	"net/http"
)

func ListConventions(
	w http.ResponseWriter,
	r *http.Request,
) {

	conventions, err := repository.GetAllConventions()

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	data := struct {
		Title       string
		Conventions []models.Convention
	}{
		Title:       "Convention Management",
		Conventions: conventions,
	}

	Render(
		w,
		"conventions.html",
		data,
	)
}

func NewConvention(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title      string
		Action     string
		Convention models.Convention
	}{
		Title:      "Add Convention",
		Action:     "/conventions/create",
		Convention: models.Convention{},
	}

	Render(
		w,
		"convention_form.html",
		data,
	)
}
func CreateConvention(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/conventions", http.StatusSeeOther)
		return
	}

	year := r.FormValue("year")
	name := r.FormValue("name")
	theme := r.FormValue("theme")
	venue := r.FormValue("venue")
	start := r.FormValue("start_date")
	end := r.FormValue("end_date")

	convention := models.Convention{
		Name:      name,
		Theme:     theme,
		Venue:     venue,
		StartDate: start,
		EndDate:   end,
	}

	// convert year
	fmt.Sscanf(year, "%d", &convention.Year)

	err := repository.CreateConvention(convention)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/conventions", http.StatusSeeOther)
}

func EditConvention(
	w http.ResponseWriter,
	r *http.Request,
) {

	id := r.URL.Query().Get("id")

	var conventionID int

	fmt.Sscanf(id, "%d", &conventionID)

	convention, err := repository.GetConventionByID(conventionID)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
	data := struct {
		Title      string
		Action     string
		Convention models.Convention
	}{
		Title:      "Edit Convention",
		Action:     "/conventions/update",
		Convention: convention,
	}

	Render(
		w,
		"convention_form.html",
		data,
	)
}

func UpdateConvention(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/conventions",
			http.StatusSeeOther,
		)

		return
	}

	var convention models.Convention

	fmt.Sscanf(r.FormValue("id"), "%d", &convention.ID)
	fmt.Sscanf(r.FormValue("year"), "%d", &convention.Year)

	convention.Name = r.FormValue("name")
	convention.Theme = r.FormValue("theme")
	convention.Venue = r.FormValue("venue")
	convention.StartDate = r.FormValue("start_date")
	convention.EndDate = r.FormValue("end_date")

	err := repository.UpdateConvention(convention)

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
		"/conventions",
		http.StatusSeeOther,
	)
}
func ActivateConvention(
	w http.ResponseWriter,
	r *http.Request,
) {

	id := r.URL.Query().Get("id")

	var conventionID int

	fmt.Sscanf(id, "%d", &conventionID)

	err := repository.ActivateConvention(conventionID)

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
		"/conventions",
		http.StatusSeeOther,
	)
}
func DeleteConvention(
	w http.ResponseWriter,
	r *http.Request,
) {

	id := r.URL.Query().Get("id")

	var conventionID int

	fmt.Sscanf(id, "%d", &conventionID)

	err := repository.DeleteConvention(conventionID)

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
		"/conventions",
		http.StatusSeeOther,
	)
}

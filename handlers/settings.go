package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

func SystemSettings(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method == http.MethodPost {

		repository.SaveSetting(
			"church_name",
			r.FormValue("church_name"),
		)

		repository.SaveSetting(
			"diocese_name",
			r.FormValue("diocese_name"),
		)

		repository.SaveSetting(
			"convention_name",
			r.FormValue("convention_name"),
		)

		repository.SaveSetting(
			"contact_email",
			r.FormValue("contact_email"),
		)

		repository.SaveSetting(
			"contact_phone",
			r.FormValue("contact_phone"),
		)

		repository.SaveSetting(
			"registration_open",
			r.FormValue("registration_open"),
		)

		http.Redirect(
			w,
			r,
			"/settings",
			http.StatusSeeOther,
		)

		return
	}

	data := struct {
		Title string

		ChurchName     string
		DioceseName    string
		ConventionName string

		ContactEmail string
		ContactPhone string

		RegistrationOpen string
	}{

		Title: "System Settings",

		ChurchName:     repository.GetSetting("church_name"),
		DioceseName:    repository.GetSetting("diocese_name"),
		ConventionName: repository.GetSetting("convention_name"),

		ContactEmail: repository.GetSetting("contact_email"),
		ContactPhone: repository.GetSetting("contact_phone"),

		RegistrationOpen: repository.GetSetting("registration_open"),
	}

	Render(
		w,
		"settings.html",
		data,
	)
}

package handlers

import (
	"net/http"

	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func EditProfile(
	w http.ResponseWriter,
	r *http.Request,
) {

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	userID, ok := session.Values["user_id"].(int)
	if !ok {

		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)

		return
	}

	switch r.Method {

	case http.MethodGet:

		user, err := services.GetUserProfile(userID)
		if err != nil {

			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)

			return
		}

		Render(
			w,
			"edit_profile.html",
			map[string]any{
				"User": user,
			},
		)

	case http.MethodPost:

		user := &models.User{

			ID: userID,

			FullName:  r.FormValue("full_name"),
			Bio:       r.FormValue("bio"),
			Location:  r.FormValue("location"),
			Website:   r.FormValue("website"),
			Gender:    r.FormValue("gender"),
			BirthDate: r.FormValue("birth_date"),
		}

		err := services.UpdateUserProfile(user)
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
			"/profile",
			http.StatusSeeOther,
		)
	}
}

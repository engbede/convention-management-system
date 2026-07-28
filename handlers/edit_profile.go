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

			// Basic Information
			FullName:   r.FormValue("full_name"),
			Bio:        r.FormValue("bio"),
			Gender:     r.FormValue("gender"),
			BirthDate:  r.FormValue("birth_date"),
			Occupation: r.FormValue("occupation"),

			// Location
			Country:  r.FormValue("country"),
			State:    r.FormValue("state"),
			Location: r.FormValue("location"),
			Website:  r.FormValue("website"),

			// Church Information
			ChurchName:  r.FormValue("church_name"),
			Circuit:     r.FormValue("circuit"),
			LocalChurch: r.FormValue("local_church"),
			Department:  r.FormValue("department"),
			Position:    r.FormValue("position"),

			// Faith Journey
			FavoriteBibleVerse: r.FormValue("favorite_bible_verse"),
			LifeVerse:          r.FormValue("life_verse"),
			Calling:            r.FormValue("calling"),
			SpiritualGifts:     r.FormValue("spiritual_gifts"),
			SalvationTestimony: r.FormValue("salvation_testimony"),

			WaterBaptized:      r.FormValue("water_baptized") != "",
			HolySpiritBaptized: r.FormValue("holy_spirit_baptized") != "",

			// Christian Interests
			FavoritePreacher:      r.FormValue("favorite_preacher"),
			FavoriteChristianBook: r.FormValue("favorite_christian_book"),
			FavoriteWorshipSong:   r.FormValue("favorite_worship_song"),
			FavoriteGospelArtist:  r.FormValue("favorite_gospel_artist"),

			// Personal
			Skills:    r.FormValue("skills"),
			Languages: r.FormValue("languages"),
			Hobbies:   r.FormValue("hobbies"),

			// Mission & Vision
			Mission:       r.FormValue("mission"),
			Vision:        r.FormValue("vision"),
			FavoriteQuote: r.FormValue("favorite_quote"),
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

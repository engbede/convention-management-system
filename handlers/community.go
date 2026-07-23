package handlers

import (
	"net/http"

	"convention-management-system/services"
)

func Community(
	w http.ResponseWriter,
	r *http.Request,
) {

	posts, err := services.GetCommunityFeed()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	data := map[string]any{

		"Posts": posts,
	}

	Render(
		w,
		"community.html",
		data,
	)
}

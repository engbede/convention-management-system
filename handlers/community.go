package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

func Community(
	w http.ResponseWriter,
	r *http.Request,
) {

	posts, err := repository.GetAllPosts()

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

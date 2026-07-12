package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/repository"
)

func ViewRegistration(w http.ResponseWriter, r *http.Request) {

	idStr := r.URL.Query().Get("id")

	id, err := strconv.Atoi(idStr)

	if err != nil {
		http.Error(w, "Invalid registration ID", http.StatusBadRequest)
		return
	}

	registration, err := repository.GetRegistrationByID(id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	Render(
		w,
		"view.html",
		registration,
	)
}

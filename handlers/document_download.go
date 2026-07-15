package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/repository"
)

func DownloadDocumentFile(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid file ID",
			http.StatusBadRequest,
		)

		return
	}

	file, err := repository.GetDocumentFileByID(id)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	http.ServeFile(
		w,
		r,
		file.FilePath,
	)
}

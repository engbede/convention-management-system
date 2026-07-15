package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/repository"
)

func UploadDocumentPage(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid document ID",
			http.StatusBadRequest,
		)

		return
	}

	document, err := repository.GetDocumentByID(id)

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
		"document_upload.html",
		document,
	)
}

package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func UploadDocumentFile(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/documents",
			http.StatusSeeOther,
		)

		return
	}

	documentID, _ := strconv.Atoi(
		r.FormValue("document_id"),
	)

	file, header, err := r.FormFile("file")

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)

		return
	}

	defer file.Close()

	err = os.MkdirAll(
		"uploads/documents",
		0755,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	filename := header.Filename

	path := filepath.Join(
		"uploads/documents",
		filename,
	)

	dst, err := os.Create(path)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	defer dst.Close()

	_, err = dst.ReadFrom(file)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	repository.SaveDocumentFile(

		models.DocumentFile{

			DocumentID: documentID,

			FileName: filename,

			FilePath: path,

			FileType: header.Header.Get("Content-Type"),
		},
	)

	http.Redirect(

		w,

		r,

		"/documents/view?id="+strconv.Itoa(documentID),

		http.StatusSeeOther,
	)
}

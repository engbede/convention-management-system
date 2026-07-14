package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func ListDocuments(
	w http.ResponseWriter,
	r *http.Request,
) {

	documents, err := repository.GetAllDocuments()

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
		"documents.html",
		documents,
	)
}

func NewDocument(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title    string
		Action   string
		Document models.Document
		Error    string
	}{
		Title:  "New Document",
		Action: "/documents/create",
	}

	Render(
		w,
		"document_form.html",
		data,
	)
}

func CreateDocument(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/documents/new",
			http.StatusSeeOther,
		)

		return
	}

	year, _ := strconv.Atoi(
		r.FormValue("year"),
	)

	document := models.Document{

		Title: r.FormValue("title"),

		Category: r.FormValue("category"),

		Convention: r.FormValue("convention"),

		Year: year,

		Description: r.FormValue("description"),

		Content: r.FormValue("content"),

		CreatedBy: "Administrator",
	}

	if document.Title == "" {

		http.Error(
			w,
			"Title is required",
			http.StatusBadRequest,
		)

		return
	}

	err := repository.CreateDocument(document)

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
		"/documents",
		http.StatusSeeOther,
	)
}
func ViewDocument(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, _ := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

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
		"document_view.html",
		document,
	)
}
func EditDocument(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, _ := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	document, err := repository.GetDocumentByID(id)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	data := struct {
		Title    string
		Action   string
		Document models.Document
		Error    string
	}{
		Title:    "Edit Document",
		Action:   "/documents/update",
		Document: document,
	}

	Render(
		w,
		"document_form.html",
		data,
	)
}
func UpdateDocument(
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

	id, _ := strconv.Atoi(
		r.FormValue("id"),
	)

	year, _ := strconv.Atoi(
		r.FormValue("year"),
	)

	document := models.Document{

		ID: id,

		Title: r.FormValue("title"),

		Category: r.FormValue("category"),

		Convention: r.FormValue("convention"),

		Year: year,

		Description: r.FormValue("description"),

		Content: r.FormValue("content"),
	}

	err := repository.UpdateDocument(document)

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
		"/documents",
		http.StatusSeeOther,
	)
}
func DeleteDocument(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, _ := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	err := repository.DeleteDocument(id)

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
		"/documents",
		http.StatusSeeOther,
	)
}

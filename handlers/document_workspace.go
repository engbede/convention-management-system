package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/repository"
)

func DocumentWorkspace(
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

	var selected interface{}
	var files interface{}

	if len(documents) > 0 {

		id := documents[0].ID

		if q := r.URL.Query().Get("id"); q != "" {

			if parsed, err := strconv.Atoi(q); err == nil {
				id = parsed
			}
		}

		document, err := repository.GetDocumentByID(id)

		if err == nil {

			selected = document

			files, _ = repository.GetDocumentFiles(id)
		}
	}

	data := struct {
		Title     string
		Documents interface{}
		Document  interface{}
		Files     interface{}
	}{
		Title:     "Document Workspace",
		Documents: documents,
		Document:  selected,
		Files:     files,
	}

	Render(
		w,
		"document_workspace.html",
		data,
	)
}

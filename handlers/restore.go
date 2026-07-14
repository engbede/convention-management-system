package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func RestoreBackup(
	w http.ResponseWriter,
	r *http.Request,
) {

	file := r.URL.Query().Get("file")

	if file == "" {

		http.Redirect(
			w,
			r,
			"/backup",
			http.StatusSeeOther,
		)

		return
	}

	source := filepath.Join(
		"backups",
		file,
	)

	src, err := os.Open(source)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	defer src.Close()

	dst, err := os.Create(
		"data/convention.db",
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	defer dst.Close()

	_, err = io.Copy(dst, src)

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
		"/backup",
		http.StatusSeeOther,
	)
}

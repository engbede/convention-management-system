package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func CreateBackup(
	w http.ResponseWriter,
	r *http.Request,
) {

	source := "data/convention.db"

	filename := time.Now().Format(
		"2006-01-02_15-04-05",
	) + ".db"

	destination := filepath.Join(
		"backups",
		filename,
	)

	err := os.MkdirAll("backups", 0755)

if err != nil {

	http.Error(
		w,
		err.Error(),
		http.StatusInternalServerError,
	)

	return
}

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

	dst, err := os.Create(destination)

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

func BackupPage(
	w http.ResponseWriter,
	r *http.Request,
) {

	files, _ := os.ReadDir("backups")

	Render(
		w,
		"backup.html",
		struct {
			Title string
			Files []os.DirEntry
		}{
			Title: "Backup & Restore",
			Files: files,
		},
	)
}

func DownloadBackup(
	w http.ResponseWriter,
	r *http.Request,
) {

	file := r.URL.Query().Get("file")

	http.ServeFile(
		w,
		r,
		filepath.Join(
			"backups",
			file,
		),
	)
}

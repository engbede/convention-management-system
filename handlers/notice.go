package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func NewNotice(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title  string
		Action string
		Notice models.Notice
		Error  string
	}{
		Title:  "Create Notice",
		Action: "/notices/create",
	}

	err := Templates.ExecuteTemplate(
		w,
		"notice_form.html",
		data,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
}
func CreateNotice(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/notices/new",
			http.StatusSeeOther,
		)

		return
	}

	notice := models.Notice{

		Title: r.FormValue("title"),

		Message: r.FormValue("message"),

		Audience: r.FormValue("audience"),

		Priority: r.FormValue("priority"),

		Pinned: r.FormValue("pinned") == "on",

		StartDate: r.FormValue("start_date"),

		EndDate: r.FormValue("end_date"),

		CreatedBy: "Administrator",
	}

	if notice.Title == "" || notice.Message == "" {

		http.Error(
			w,
			"Title and message are required.",
			http.StatusBadRequest,
		)

		return
	}

	err := repository.CreateNotice(notice)

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
		"/notices",
		http.StatusSeeOther,
	)
}
func ListNotices(
	w http.ResponseWriter,
	r *http.Request,
) {

	notices, err := repository.GetAllNotices()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	err = Templates.ExecuteTemplate(
		w,
		"notices.html",
		notices,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}
}
func ViewNotice(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, _ := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	notice, err := repository.GetNoticeByID(id)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	err = Templates.ExecuteTemplate(
		w,
		"notice_view.html",
		notice,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}
}

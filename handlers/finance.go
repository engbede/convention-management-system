package handlers

import "net/http"

func FinanceDashboard(
	w http.ResponseWriter,
	r *http.Request,
) {

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

func ListFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title string
	}{
		Title: "Finance",
	}

	Render(
		w,
		"finance.html",
		data,
	)
}
func NewFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

func CreateFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

func EditFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

func UpdateFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
}

func DeleteFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	http.Redirect(
		w,
		r,
		"/dashboard",
		http.StatusSeeOther,
	)
	
}

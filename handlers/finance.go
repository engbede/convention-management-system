package handlers

import (
	"log"
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func NewFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title string
	}{
		Title: "New Finance Record",
	}

	Render(
		w,
		"finance_form.html",
		data,
	)
}

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

	finances, err := repository.GetAllFinance()

	if err != nil {
		log.Println("GetAllFinance error:", err)

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}
	income,
		expense,
		balance,
		totalTransactions,
		err := repository.GetFinanceSummary()

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	data := struct {
		Title             string
		Finances          interface{}
		Income            float64
		Expense           float64
		Balance           float64
		TotalTransactions int
	}{
		Title:             "Finance",
		Finances:          finances,
		Income:            income,
		Expense:           expense,
		Balance:           balance,
		TotalTransactions: totalTransactions,
	}

	Render(
		w,
		"finance.html",
		data,
	)
}

func CreateFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/finance/new",
			http.StatusSeeOther,
		)
		return
	}

	amount, err := strconv.ParseFloat(
		r.FormValue("amount"),
		64,
	)

	if err != nil {
		http.Error(
			w,
			"Invalid amount",
			http.StatusBadRequest,
		)
		return
	}

	finance := models.Finance{
		Type:        r.FormValue("type"),
		Category:    r.FormValue("category"),
		Description: r.FormValue("description"),
		Amount:      amount,
		RecordedBy:  r.FormValue("recorded_by"),
		Date:        r.FormValue("date"),
	}

	err = repository.CreateFinance(finance)

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
		"/finance",
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

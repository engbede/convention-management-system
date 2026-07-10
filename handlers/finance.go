package handlers

import (
	"net/http"

	"convention-management-system/repository"
)

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

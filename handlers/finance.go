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
		Title   string
		Action  string
		Finance models.Finance
	}{
		Title:   "New Finance Record",
		Action:  "/finance/create",
		Finance: models.Finance{},
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

	id, err := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid finance ID",
			http.StatusBadRequest,
		)

		return
	}

	finance, err := repository.GetFinanceByID(id)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	data := struct {
		Title   string
		Action  string
		Finance models.Finance
	}{
		Title:   "Edit Finance Record",
		Action:  "/finance/update",
		Finance: finance,
	}

	Render(
		w,
		"finance_form.html",
		data,
	)
}

func UpdateFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/finance",
			http.StatusSeeOther,
		)

		return
	}

	id, err := strconv.Atoi(
		r.FormValue("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid finance ID",
			http.StatusBadRequest,
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

		ID: id,

		Type: r.FormValue("type"),

		Category: r.FormValue("category"),

		Description: r.FormValue("description"),

		Amount: amount,

		RecordedBy: r.FormValue("recorded_by"),

		Date: r.FormValue("date"),
	}

	err = repository.UpdateFinance(finance)

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

func DeleteFinance(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid finance ID",
			http.StatusBadRequest,
		)

		return
	}

	err = repository.DeleteFinance(id)

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

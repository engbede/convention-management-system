package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func ContactPage(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title string
		Query map[string]string
	}{
		Title: "Contact Us",
		Query: map[string]string{
			"success": r.URL.Query().Get("success"),
		},
	}

	Render(
		w,
		"contact.html",
		data,
	)
}

func SubmitInquiry(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/contact",
			http.StatusSeeOther,
		)

		return
	}

	inquiry := models.Inquiry{

		Name: r.FormValue("name"),

		Phone: r.FormValue("phone"),

		Email: r.FormValue("email"),

		Subject: r.FormValue("subject"),

		Message: r.FormValue("message"),
	}

	err := repository.CreateInquiry(inquiry)

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
		"/contact?success=1",
		http.StatusSeeOther,
	)
}

func ListInquiries(
	w http.ResponseWriter,
	r *http.Request,
) {

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	page := 1

	if p := r.URL.Query().Get("page"); p != "" {

		value, err := strconv.Atoi(p)

		if err == nil && value > 0 {
			page = value
		}
	}

	const pageSize = 20

	inquiries, err := repository.FilterInquiries(
		search,
		status,
		page,
		pageSize,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	stats, err := repository.GetInquiryStats()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	totalRecords, err := repository.CountInquiries(
		search,
		status,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	totalPages := (totalRecords + pageSize - 1) / pageSize

	Render(
		w,
		"inquiries.html",
		struct {
			Title        string
			Inquiries    []models.Inquiry
			Stats        models.InquiryStats
			Search       string
			Status       string
			Page         int
			PageSize     int
			TotalRecords int
			TotalPages   int
		}{
			Title:        "Contact Inquiries",
			Inquiries:    inquiries,
			Stats:        stats,
			Search:       search,
			Status:       status,
			Page:         page,
			PageSize:     pageSize,
			TotalRecords: totalRecords,
			TotalPages:   totalPages,
		},
	)
}

func ViewInquiry(
	w http.ResponseWriter,
	r *http.Request,
) {

	id, err := strconv.Atoi(
		r.URL.Query().Get("id"),
	)

	if err != nil {

		http.NotFound(
			w,
			r,
		)

		return
	}

	inquiry, err := repository.GetInquiryByID(id)

	replies, err := repository.GetRepliesByInquiry(id)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
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
		"inquiry_view.html",
		struct {
			Title   string
			Inquiry models.Inquiry
			Replies []models.InquiryReply
		}{
			Title:   "Inquiry Details",
			Inquiry: inquiry,
			Replies: replies,
		},
	)
}

func UpdateInquiryStatus(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/admin/inquiries",
			http.StatusSeeOther,
		)

		return
	}

	id, err := strconv.Atoi(
		r.FormValue("id"),
	)

	if err != nil {

		http.NotFound(
			w,
			r,
		)

		return
	}

	status := r.FormValue("status")

	switch status {
	case "Pending", "In Progress", "Resolved":
		// valid
	default:
		status = "Resolved"
	}

	err = repository.UpdateInquiryStatus(
		id,
		status,
	)

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
		"/admin/inquiries",
		http.StatusSeeOther,
	)
}

func DeleteInquiry(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/admin/inquiries",
			http.StatusSeeOther,
		)

		return
	}

	id, err := strconv.Atoi(
		r.FormValue("id"),
	)

	if err != nil {

		http.NotFound(
			w,
			r,
		)

		return
	}

	err = repository.DeleteInquiry(id)

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
		"/admin/inquiries",
		http.StatusSeeOther,
	)
}

func SubmitInquiryReply(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/admin/inquiries",
			http.StatusSeeOther,
		)

		return
	}

	inquiryID, err := strconv.Atoi(
		r.FormValue("inquiry_id"),
	)

	if err != nil {

		http.NotFound(
			w,
			r,
		)

		return
	}

	reply := models.InquiryReply{

		InquiryID: inquiryID,

		AdminName: "Administrator",

		Message: r.FormValue("message"),
	}

	if reply.Message == "" {

		http.Redirect(
			w,
			r,
			"/admin/inquiry?id="+strconv.Itoa(inquiryID),
			http.StatusSeeOther,
		)

		return
	}

	err = repository.CreateInquiryReply(reply)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	err = repository.UpdateInquiryStatus(
		inquiryID,
		"Resolved",
	)

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
		"/admin/inquiry?id="+strconv.Itoa(inquiryID),
		http.StatusSeeOther,
	)
}

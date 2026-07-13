package handlers

import (
	"net/http"
)

func SendEmailBroadcast(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/communication/email",
			http.StatusSeeOther,
		)
		return
	}

	group := r.FormValue("group")
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	Render(
		w,
		"success.html",
		struct {
			Title   string
			Message string
			Group   string
			Subject string
			Body    string
		}{
			Title:   "Email Broadcast",
			Message: "Email broadcast queued successfully.",
			Group:   group,
			Subject: subject,
			Body:    message,
		},
	)
}

func SendSMSBroadcast(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/communication/sms",
			http.StatusSeeOther,
		)
		return
	}

	group := r.FormValue("group")
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	Render(
		w,
		"success.html",
		struct {
			Title   string
			Message string
			Group   string
			Subject string
			Body    string
		}{
			Title:   "SMS Broadcast",
			Message: "SMS broadcast queued successfully.",
			Group:   group,
			Subject: subject,
			Body:    message,
		},
	)
}

func SendEmergencyNotice(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/communication/emergency",
			http.StatusSeeOther,
		)
		return
	}

	group := r.FormValue("group")
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	Render(
		w,
		"success.html",
		struct {
			Title   string
			Message string
			Group   string
			Subject string
			Body    string
		}{
			Title:   "Emergency Notice",
			Message: "Emergency notice sent successfully.",
			Group:   group,
			Subject: subject,
			Body:    message,
		},
	)
}

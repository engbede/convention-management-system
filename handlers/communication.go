package handlers

import "net/http"

func Communication(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title string
	}{
		Title: "Communication Centre",
	}

	Render(
		w,
		"communication.html",
		data,
	)
}
func EmailBroadcast(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title  string
		Action string
	}{
		Title:  "Email Broadcast",
		Action: "/communication/email/send",
	}

	Render(
		w,
		"email.html",
		data,
	)
}

func SMSBroadcast(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title  string
		Action string
	}{
		Title:  "SMS Broadcast",
		Action: "/communication/sms/send",
	}

	Render(
		w,
		"sms.html",
		data,
	)
}

func EmergencyNotice(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title  string
		Action string
	}{
		Title:  "Emergency Notice",
		Action: "/communication/emergency/send",
	}

	Render(
		w,
		"emergency.html",
		data,
	)
}

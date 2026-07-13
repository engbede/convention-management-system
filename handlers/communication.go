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
func EmailBroadcast(w http.ResponseWriter, r *http.Request) {
	Render(w, "email.html", struct{ Title string }{
		Title: "Email Broadcast",
	})
}

func SMSBroadcast(w http.ResponseWriter, r *http.Request) {
	Render(w, "sms.html", struct{ Title string }{
		Title: "SMS Broadcast",
	})
}

func EmergencyNotice(w http.ResponseWriter, r *http.Request) {
	Render(w, "emergency.html", struct{ Title string }{
		Title: "Emergency Notice",
	})
}

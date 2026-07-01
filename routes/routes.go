package routes

import (
	"net/http"

	"convention-management-system/handlers"
	"convention-management-system/middleware"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", handlers.ShowForm)
	mux.HandleFunc("/register", handlers.Register)

	mux.HandleFunc(
		"/registrations",
		middleware.RequireAuth(
			handlers.ListRegistrations,
		),
	)
	mux.HandleFunc(
		"/dashboard",
		middleware.RequireAuth(
			handlers.Dashboard,
		),
	)

	mux.HandleFunc(
		"/view",
		middleware.RequireAuth(
			handlers.ViewRegistration,
		),
	)
	mux.HandleFunc(
		"/idcard",
		middleware.RequireAuth(
			handlers.IDCard,
		),
	)
	mux.HandleFunc(
		"/edit",
		middleware.RequireAuth(
			handlers.EditRegistration,
		),
	)
	mux.HandleFunc(
		"/update",
		middleware.RequireAuth(
			handlers.UpdateRegistration,
		),
	)
	mux.HandleFunc(
		"/checkin",
		middleware.RequireAuth(
			handlers.CheckIn,
		),
	)
	mux.HandleFunc(
		"/delete",
		middleware.RequireAuth(
			handlers.DeleteRegistration,
		),
	)
	mux.HandleFunc(
		"/logout",
		middleware.RequireAuth(
			handlers.Logout,
		),
	)
	mux.HandleFunc(
		"/export/excel",
		middleware.RequireAuth(
			handlers.ExportExcel,
		),
	)
	mux.HandleFunc(
		"/export/pdf",
		middleware.RequireAuth(
			handlers.ExportPDF,
		),
	)
	mux.HandleFunc(
		"/print-idcards",
		middleware.RequireAuth(
			handlers.PrintIDCards,
		),
	)
	mux.HandleFunc("/login", handlers.Login)

	mux.HandleFunc("/healthz", handlers.Health,)

	fs := http.FileServer(http.Dir("static"))

	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", fs),
	)

}

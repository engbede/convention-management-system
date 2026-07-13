package routes

import (
	"net/http"

	"convention-management-system/handlers"
	"convention-management-system/middleware"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", handlers.Home)

	mux.HandleFunc("/register", handlers.ShowForm)

	mux.HandleFunc("/submit-registration", handlers.Register)

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
		"/communication",
		middleware.RequireAuth(
			handlers.Communication,
		),
	)

	// Communication Centre
	mux.HandleFunc(
		"/communication/email",
		middleware.RequireAuth(
			handlers.EmailBroadcast,
		),
	)

	mux.HandleFunc(
		"/communication/email/send",
		middleware.RequireAuth(
			handlers.SendEmailBroadcast,
		),
	)

	mux.HandleFunc(
		"/communication/sms",
		middleware.RequireAuth(
			handlers.SMSBroadcast,
		),
	)

	mux.HandleFunc(
		"/communication/sms/send",
		middleware.RequireAuth(
			handlers.SendSMSBroadcast,
		),
	)

	mux.HandleFunc(
		"/communication/emergency",
		middleware.RequireAuth(
			handlers.EmergencyNotice,
		),
	)

	mux.HandleFunc(
		"/communication/emergency/send",
		middleware.RequireAuth(
			handlers.SendEmergencyNotice,
		),
	)
	mux.HandleFunc(
		"/reports",
		handlers.Reports,
	)
	// Convention Management
	mux.HandleFunc(
		"/conventions",
		middleware.RequireAuth(handlers.ListConventions),
	)

	mux.HandleFunc(
		"/conventions/new",
		middleware.RequireAuth(handlers.NewConvention),
	)

	mux.HandleFunc(
		"/conventions/create",
		middleware.RequireAuth(handlers.CreateConvention),
	)

	mux.HandleFunc(
		"/conventions/edit",
		middleware.RequireAuth(handlers.EditConvention),
	)

	mux.HandleFunc(
		"/conventions/update",
		middleware.RequireAuth(handlers.UpdateConvention),
	)

	mux.HandleFunc(
		"/conventions/activate",
		middleware.RequireAuth(handlers.ActivateConvention),
	)
	mux.HandleFunc(
		"/conventions/delete",
		middleware.RequireAuth(
			handlers.DeleteConvention,
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

	// Officials Management
	mux.HandleFunc(
		"/officials",
		middleware.RequireAuth(
			handlers.ListOfficials,
		),
	)

	mux.HandleFunc(
		"/officials/new",
		middleware.RequireAuth(
			handlers.ShowOfficialForm,
		),
	)

	mux.HandleFunc(
		"/officials/create",
		middleware.RequireAuth(
			handlers.CreateOfficial,
		),
	)

	mux.HandleFunc(
		"/officials/view",
		middleware.RequireAuth(
			handlers.ViewOfficial,
		),
	)

	mux.HandleFunc(
		"/officials/edit",
		middleware.RequireAuth(
			handlers.EditOfficial,
		),
	)

	mux.HandleFunc(
		"/officials/update",
		middleware.RequireAuth(
			handlers.UpdateOfficial,
		),
	)

	mux.HandleFunc(
		"/officials/delete",
		middleware.RequireAuth(
			handlers.DeleteOfficial,
		),
	)
	mux.HandleFunc(
		"/notices/new",
		middleware.RequireAuth(
			handlers.NewNotice,
		),
	)
	mux.HandleFunc(
		"/notices/create",
		middleware.RequireAuth(
			handlers.CreateNotice,
		),
	)
	mux.HandleFunc(
		"/notices",
		middleware.RequireAuth(
			handlers.ListNotices,
		),
	)
	mux.HandleFunc(
		"/notices/view",
		middleware.RequireAuth(
			handlers.ViewNotice,
		),
	)
	mux.HandleFunc(
		"/notices/edit",
		middleware.RequireAuth(
			handlers.EditNotice,
		),
	)

	mux.HandleFunc(
		"/notices/update",
		middleware.RequireAuth(
			handlers.UpdateNotice,
		),
	)

	mux.HandleFunc(
		"/notices/delete",
		middleware.RequireAuth(
			handlers.DeleteNotice,
		),
	)

	mux.HandleFunc(
		"/communication/email",
		middleware.RequireAuth(handlers.EmailBroadcast),
	)

	mux.HandleFunc(
		"/communication/sms",
		middleware.RequireAuth(handlers.SMSBroadcast),
	)

	mux.HandleFunc(
		"/communication/emergency",
		middleware.RequireAuth(handlers.EmergencyNotice),
	)

	mux.HandleFunc("/finance", handlers.ListFinance)

	mux.HandleFunc("/finance/new", handlers.NewFinance)

	mux.HandleFunc("/finance/create", handlers.CreateFinance)

	mux.HandleFunc("/finance/edit", handlers.EditFinance)

	mux.HandleFunc("/finance/update", handlers.UpdateFinance)

	mux.HandleFunc("/finance/delete", handlers.DeleteFinance)

	mux.HandleFunc("/login", handlers.Login)

	mux.HandleFunc("/healthz", handlers.Health)

	fs := http.FileServer(http.Dir("static"))

	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", fs),
	)

}

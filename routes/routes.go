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
		"/admin/logout",
		middleware.RequireAuth(
			handlers.AdminLogout,
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
		"/backup",
		middleware.RequireAuth(
			handlers.BackupPage,
		),
	)

	mux.HandleFunc(
		"/backup/create",
		middleware.RequireAuth(
			handlers.CreateBackup,
		),
	)

	mux.HandleFunc(
		"/backup/download",
		middleware.RequireAuth(
			handlers.DownloadBackup,
		),
	)
	mux.HandleFunc(
		"/backup/restore",
		middleware.RequireAuth(
			handlers.RestoreBackup,
		),
	)
	mux.HandleFunc(
		"/settings",
		middleware.RequireAuth(
			handlers.SystemSettings,
		),
	)
	// Documentation & Minutes
	mux.HandleFunc(
		"/documents",
		middleware.RequireAuth(
			handlers.ListDocuments,
		),
	)

	mux.HandleFunc(
		"/documents/new",
		middleware.RequireAuth(
			handlers.NewDocument,
		),
	)

	mux.HandleFunc(
		"/documents/create",
		middleware.RequireAuth(
			handlers.CreateDocument,
		),
	)

	mux.HandleFunc(
		"/documents/view",
		middleware.RequireAuth(
			handlers.ViewDocument,
		),
	)

	mux.HandleFunc(
		"/documents/edit",
		middleware.RequireAuth(
			handlers.EditDocument,
		),
	)

	mux.HandleFunc(
		"/documents/update",
		middleware.RequireAuth(
			handlers.UpdateDocument,
		),
	)

	mux.HandleFunc(
		"/documents/delete",
		middleware.RequireAuth(
			handlers.DeleteDocument,
		),
	)
	mux.HandleFunc(
		"/documents/upload",
		middleware.RequireAuth(
			handlers.UploadDocumentFile,
		),
	)
	mux.HandleFunc(
		"/documents/download",
		middleware.RequireAuth(
			handlers.DownloadDocumentFile,
		),
	)
	mux.HandleFunc(
		"/documents/workspace",
		middleware.RequireAuth(
			handlers.DocumentWorkspace,
		),
	)
	mux.HandleFunc(
		"/documents/attach",
		middleware.RequireAuth(
			handlers.UploadDocumentPage,
		),
	)
	mux.HandleFunc(
		"/contact",
		handlers.ContactPage,
	)

	mux.HandleFunc(
		"/contact/send",
		handlers.SubmitInquiry,
	)

	mux.HandleFunc(
		"/admin/inquiries",
		middleware.RequireAuth(
			handlers.ListInquiries,
		),
	)

	mux.HandleFunc(
		"/admin/inquiry",
		middleware.RequireAuth(
			handlers.ViewInquiry,
		),
	)

	mux.HandleFunc(
		"/admin/inquiry/status",
		middleware.RequireAuth(
			handlers.UpdateInquiryStatus,
		),
	)

	mux.HandleFunc(
		"/admin/inquiry/delete",
		middleware.RequireAuth(
			handlers.DeleteInquiry,
		),
	)

	mux.HandleFunc(
		"/admin/inquiry/reply",
		middleware.RequireAuth(handlers.SubmitInquiryReply),
	)

	mux.HandleFunc(
		"/test-sms",
		handlers.TestSMS,
	)
	mux.HandleFunc(
		"/profile",
		middleware.RequireLogin(
			handlers.Profile,
		),
	)

	mux.HandleFunc(
		"/friends",
		handlers.FriendsDashboard,
	)

	mux.HandleFunc(
		"/discover",
		handlers.Discover,
	)

	mux.HandleFunc(
		"/friend/request",
		handlers.SendFriendRequest,
	)

	mux.HandleFunc(
		"/user/",
		middleware.RequireLogin(
			handlers.PublicProfile,
		),
	)

	mux.HandleFunc(
		"/profile/edit",
		middleware.RequireLogin(
			handlers.EditProfile,
		),
	)

	mux.HandleFunc(
		"/profile/upload-photo",
		middleware.RequireLogin(
			handlers.UploadProfilePhoto,
		),
	)

	mux.HandleFunc(
		"/profile/upload-cover",
		middleware.RequireLogin(
			handlers.UploadCoverPhoto,
		),
	)

	mux.HandleFunc(
		"/follow",
		middleware.RequireLogin(
			handlers.FollowUser,
		),
	)

	mux.HandleFunc(
		"/unfollow",
		middleware.RequireLogin(
			handlers.UnfollowUser,
		),
	)

	mux.HandleFunc(
		"/notifications",
		middleware.RequireLogin(
			handlers.Notifications,
		),
	)

	mux.HandleFunc(
		"/community",
		middleware.RequireLogin(
			handlers.Community,
		),
	)

	mux.HandleFunc(
		"/community/post",
		middleware.RequireLogin(
			handlers.CreateCommunityPost,
		),
	)

	mux.HandleFunc(
		"/community/comment",
		middleware.RequireLogin(
			handlers.CreateComment,
		),
	)

	mux.HandleFunc(
		"/community/react",
		handlers.ReactToPost,
	)

	mux.HandleFunc(
		"/community/login",
		handlers.CommunityLogin,
	)

	mux.HandleFunc(
		"/community/logout",
		handlers.CommunityLogout,
	)

	mux.HandleFunc("/friends/request", handlers.SendFriendRequest)
	mux.HandleFunc("/friends/accept", handlers.AcceptFriendRequest)
	mux.HandleFunc("/friends/decline", handlers.DeclineFriendRequest)
	mux.HandleFunc("/friends/cancel", handlers.CancelFriendRequest)
	mux.HandleFunc("/friends/remove", handlers.RemoveFriend)
	mux.HandleFunc("/signup", handlers.ShowSignup)

	mux.HandleFunc("/create-account", handlers.Signup)

	mux.HandleFunc("/finance", handlers.ListFinance)

	mux.HandleFunc("/finance/new", handlers.NewFinance)

	mux.HandleFunc("/finance/create", handlers.CreateFinance)

	mux.HandleFunc("/finance/edit", handlers.EditFinance)

	mux.HandleFunc("/finance/update", handlers.UpdateFinance)

	mux.HandleFunc("/finance/delete", handlers.DeleteFinance)

	mux.HandleFunc("/admin/login", handlers.AdminLogin)

	mux.HandleFunc("/healthz", handlers.Health)

	fs := http.FileServer(http.Dir("static"))

	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", fs),
	)

	uploads := http.FileServer(http.Dir("uploads"))

	mux.Handle(
		"/uploads/",
		http.StripPrefix("/uploads/", uploads),
	)
}

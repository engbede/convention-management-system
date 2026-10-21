package routes

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"convention-management-system/handlers"
	"convention-management-system/middleware"
	"convention-management-system/repository"
	"convention-management-system/utils"
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
		middleware.RequireAuth(
			handlers.Reports,
		),
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
		handlers.AdminLogout,
	)
	mux.HandleFunc(
		"/logout",
		handlers.AdminLogout,
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
		"/announcements",
		handlers.PublicAnnouncements,
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
		"/backup/delete",
		middleware.RequireAuth(
			handlers.DeleteBackup,
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
		handlers.DownloadDocumentFile,
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
		middleware.RequireLogin(
			handlers.FriendsDashboard,
		),
	)

	mux.HandleFunc(
		"/friend-requests",
		middleware.RequireLogin(
			handlers.FriendRequests,
		),
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
		"/api/notifications/read",
		middleware.RequireLogin(
			handlers.MarkNotificationsReadAPI,
		),
	)

	mux.HandleFunc(
		"/notifications/read-all",
		middleware.RequireLogin(
			handlers.MarkNotificationsReadAPI,
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

	// Resilient Chunked Video Upload APIs (supports up to 10 GB long-form recordings)
	mux.HandleFunc("/api/upload/config", handlers.GetUploadConfigAPI)
	mux.HandleFunc("/api/upload/initiate", middleware.RequireLogin(handlers.InitiateUploadAPI))
	mux.HandleFunc("/api/upload/chunk", middleware.RequireLogin(handlers.UploadChunkAPI))
	mux.HandleFunc("/api/upload/complete", middleware.RequireLogin(handlers.CompleteUploadAPI))
	mux.HandleFunc("/api/upload/abort", middleware.RequireLogin(handlers.AbortUploadAPI))
	mux.HandleFunc("/api/upload/status", middleware.RequireLogin(handlers.GetUploadStatusAPI))

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
		"/community/post/reactions",
		handlers.GetPostReactionsAPI,
	)

	mux.HandleFunc(
		"/community/login",
		handlers.CommunityLogin,
	)

	mux.HandleFunc(
		"/community/logout",
		handlers.CommunityLogout,
	)

	mux.HandleFunc(
		"/community/prayer/submit",
		middleware.RequireLogin(handlers.SubmitCommunityPrayerRequest),
	)
	mux.HandleFunc(
		"/community/prayer/pray",
		middleware.RequireLogin(handlers.TogglePrayerIntercessionAPI),
	)

	// Public Policy & Community Guidelines
	mux.HandleFunc(
		"/community/guidelines",
		handlers.CommunityGuidelinesHandler,
	)
	mux.HandleFunc(
		"/guidelines",
		handlers.CommunityGuidelinesHandler,
	)
	mux.HandleFunc(
		"/privacy",
		handlers.PrivacyPolicyHandler,
	)
	mux.HandleFunc(
		"/terms",
		handlers.TermsOfUseHandler,
	)
	mux.HandleFunc(
		"/terms/status",
		middleware.RequireLogin(
			handlers.TermsStatusAPI,
		),
	)
	mux.HandleFunc(
		"/terms/accept",
		middleware.RequireLogin(
			handlers.AcceptTermsAPI,
		),
	)
	mux.HandleFunc(
		"/terms/decline",
		handlers.DeclineTermsAPI,
	)

	// Youth Community Chat & Messaging
	mux.HandleFunc(
		"/messages",
		middleware.RequireLogin(
			handlers.ChatInbox,
		),
	)
	mux.HandleFunc(
		"/chat",
		middleware.RequireLogin(
			handlers.ChatInbox,
		),
	)
	mux.HandleFunc(
		"/messages/start",
		middleware.RequireLogin(
			handlers.StartDirectChat,
		),
	)
	mux.HandleFunc(
		"/messages/send",
		middleware.RequireLogin(
			handlers.SendMessageAPI,
		),
	)
	mux.HandleFunc(
		"/messages/history",
		middleware.RequireLogin(
			handlers.GetMessagesAPI,
		),
	)
	mux.HandleFunc(
		"/messages/read",
		middleware.RequireLogin(
			handlers.MarkReadAPI,
		),
	)
	mux.HandleFunc(
		"/messages/mute",
		middleware.RequireLogin(
			handlers.ToggleMuteAPI,
		),
	)
	mux.HandleFunc(
		"/messages/disappearing",
		middleware.RequireLogin(
			handlers.SetDisappearingMessagesAPI,
		),
	)
	mux.HandleFunc(
		"/messages/archive",
		middleware.RequireLogin(
			handlers.ToggleArchiveAPI,
		),
	)
	mux.HandleFunc(
		"/messages/upload",
		middleware.RequireLogin(
			handlers.UploadAttachmentAPI,
		),
	)
	mux.HandleFunc(
		"/messages/block",
		middleware.RequireLogin(
			handlers.BlockUserAPI,
		),
	)
	mux.HandleFunc(
		"/messages/unblock",
		middleware.RequireLogin(
			handlers.UnblockUserAPI,
		),
	)
	mux.HandleFunc(
		"/messages/report",
		middleware.RequireLogin(
			handlers.ReportAbuseAPI,
		),
	)
	mux.HandleFunc(
		"/messages/blocked",
		middleware.RequireLogin(
			handlers.GetBlockedUsersAPI,
		),
	)
	mux.HandleFunc(
		"/messages/moderation/reports",
		middleware.RequireLogin(
			handlers.GetModerationReportsAPI,
		),
	)
	mux.HandleFunc(
		"/messages/moderation/reports/resolve",
		middleware.RequireLogin(
			handlers.ResolveModerationReportAPI,
		),
	)
	mux.HandleFunc(
		"/messages/search",
		middleware.RequireLogin(
			handlers.SearchMessagesAPI,
		),
	)
	mux.HandleFunc(
		"/messages/media",
		middleware.RequireLogin(
			handlers.GetSharedMediaAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/ice-servers",
		middleware.RequireLogin(
			handlers.ICEServersAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/log",
		middleware.RequireLogin(
			handlers.LogCallAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/link/generate",
		middleware.RequireLogin(
			handlers.GenerateCallLinkAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/join",
		middleware.RequireLogin(
			handlers.JoinCallLinkHandler,
		),
	)
	mux.HandleFunc(
		"/messages/call/status",
		middleware.RequireLogin(
			handlers.GetCallStatusAPI,
		),
	)
	mux.HandleFunc(
		"/push/vapid-public-key",
		handlers.GetVAPIDPublicKeyAPI,
	)
	mux.HandleFunc(
		"/push/subscribe",
		middleware.RequireLogin(
			handlers.SubscribePushAPI,
		),
	)
	mux.HandleFunc(
		"/push/unsubscribe",
		middleware.RequireLogin(
			handlers.UnsubscribePushAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/create",
		middleware.RequireLogin(
			handlers.CreateGroupAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/join",
		middleware.RequireLogin(
			handlers.JoinGroupAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/leave",
		middleware.RequireLogin(
			handlers.LeaveGroupAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/members/add",
		middleware.RequireLogin(
			handlers.AddGroupMembersAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/member/add",
		middleware.RequireLogin(
			handlers.AddGroupMembersAPI,
		),
	)
	mux.HandleFunc(
		"/messages/direct/add-members",
		middleware.RequireLogin(
			handlers.ConvertDirectToGroupAPI,
		),
	)
	mux.HandleFunc(
		"/messages/convert-to-group",
		middleware.RequireLogin(
			handlers.ConvertDirectToGroupAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/members/remove",
		middleware.RequireLogin(
			handlers.RemoveGroupMemberAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/members/role",
		middleware.RequireLogin(
			handlers.UpdateGroupMemberRoleAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/update",
		middleware.RequireLogin(
			handlers.UpdateGroupSettingsAPI,
		),
	)
	mux.HandleFunc(
		"/messages/group/settings/update",
		middleware.RequireLogin(
			handlers.UpdateGroupSettingsAPI,
		),
	)
	mux.HandleFunc(
		"/messages/groups/discover",
		middleware.RequireLogin(
			handlers.DiscoverGroupsAPI,
		),
	)
	mux.HandleFunc(
		"/messages/groups/requests/submit",
		middleware.RequireLogin(
			handlers.SubmitGroupMembershipRequestAPI,
		),
	)
	mux.HandleFunc(
		"/messages/groups/requests/review",
		middleware.RequireLogin(
			handlers.ReviewGroupMembershipRequestAPI,
		),
	)
	mux.HandleFunc(
		"/messages/groups/requests/cancel",
		middleware.RequireLogin(
			handlers.CancelGroupMembershipRequestAPI,
		),
	)
	mux.HandleFunc(
		"/messages/groups/requests",
		middleware.RequireLogin(
			handlers.GetGroupMembershipRequestsAPI,
		),
	)
	mux.HandleFunc(
		"/messages/message/delete",
		middleware.RequireLogin(
			handlers.DeleteMessageAPI,
		),
	)
	mux.HandleFunc(
		"/messages/delete",
		middleware.RequireLogin(
			handlers.DeleteMessageAPI,
		),
	)
	mux.HandleFunc(
		"/messages/clear",
		middleware.RequireLogin(
			handlers.ClearChatAPI,
		),
	)
	mux.HandleFunc(
		"/messages/pin",
		middleware.RequireLogin(
			handlers.TogglePinAPI,
		),
	)
	mux.HandleFunc(
		"/messages/export",
		middleware.RequireLogin(
			handlers.ExportChatAPI,
		),
	)
	mux.HandleFunc(
		"/messages/members",
		middleware.RequireLogin(
			handlers.GetConversationMembersAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/start",
		middleware.RequireLogin(
			handlers.StartGroupCallAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/join",
		middleware.RequireLogin(
			handlers.JoinGroupCallAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/leave",
		middleware.RequireLogin(
			handlers.LeaveGroupCallAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/active",
		middleware.RequireLogin(
			handlers.GetActiveGroupCallAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/mute",
		middleware.RequireLogin(
			handlers.MuteGroupCallParticipantAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/kick",
		middleware.RequireLogin(
			handlers.KickGroupCallParticipantAPI,
		),
	)
	mux.HandleFunc(
		"/messages/call/group/end",
		middleware.RequireLogin(
			handlers.EndGroupCallAPI,
		),
	)
	mux.HandleFunc(
		"/ws/chat",
		handlers.ChatWebSocketHandler,
	)

	mux.HandleFunc("/friends/request", handlers.SendFriendRequest)
	mux.HandleFunc("/friends/accept", handlers.AcceptFriendRequest)
	mux.HandleFunc("/friends/decline", handlers.DeclineFriendRequest)
	mux.HandleFunc("/friends/cancel", handlers.CancelFriendRequest)
	mux.HandleFunc("/friends/remove", handlers.RemoveFriend)

	// Aliases for template compatibility
	mux.HandleFunc("/friend/accept", handlers.AcceptFriendRequest)
	mux.HandleFunc("/friend/decline", handlers.DeclineFriendRequest)
	mux.HandleFunc("/friend/cancel", handlers.CancelFriendRequest)
	mux.HandleFunc("/friend/remove", handlers.RemoveFriend)
	mux.HandleFunc("/signup", handlers.ShowSignup)
	mux.HandleFunc("/groups", middleware.RequireLogin(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/messages", http.StatusSeeOther)
	}))
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/#conventions", http.StatusSeeOther)
	})
	mux.HandleFunc("/prayer", middleware.RequireLogin(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/messages", http.StatusSeeOther)
	}))
	mux.HandleFunc("/bible-study", middleware.RequireLogin(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/messages", http.StatusSeeOther)
	}))
	mux.HandleFunc("/publications", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/announcements", http.StatusSeeOther)
	})
	mux.HandleFunc("/saved", middleware.RequireLogin(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
	}))

	// Live Streaming & Online Programmes
	mux.HandleFunc("/programmes", handlers.ListProgrammes)
	mux.HandleFunc("/programmes/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/programmes/" {
			http.Redirect(w, r, "/programmes", http.StatusMovedPermanently)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/live", handlers.PublicLiveViewing)
	mux.HandleFunc("/live-programmes", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/programmes", http.StatusSeeOther)
	})
	mux.HandleFunc("/online-programmes", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/programmes", http.StatusSeeOther)
	})
	mux.HandleFunc("/programmes/view", handlers.ViewProgramme)
	mux.HandleFunc("/programmes/new", handlers.NewProgramme)
	mux.HandleFunc("/programmes/create", handlers.CreateProgramme)
	mux.HandleFunc("/programmes/edit", handlers.EditProgramme)
	mux.HandleFunc("/programmes/update", handlers.UpdateProgramme)
	mux.HandleFunc("/programmes/status", handlers.TransitionProgrammeStatusHandler)
	mux.HandleFunc("/programmes/chat/send", handlers.SendProgrammeChatMessage)
	mux.HandleFunc("/programmes/chat/messages", handlers.GetProgrammeChatMessagesAPI)
	mux.HandleFunc("/programmes/chat/hide", handlers.HideProgrammeChatMessage)
	mux.HandleFunc("/programmes/prayer/submit", handlers.SubmitPrayerRequest)
	mux.HandleFunc("/programmes/prayer/list", handlers.GetProgrammePrayerRequestsAPI)
	mux.HandleFunc("/programmes/prayer/status", handlers.UpdatePrayerRequestStatusHandler)
	mux.HandleFunc("/programmes/moderators/add", handlers.AssignModeratorHandler)
	mux.HandleFunc("/programmes/moderators/remove", handlers.RemoveModeratorHandler)

	mux.HandleFunc("/create-account", handlers.Signup)
	mux.HandleFunc("/forgot-password", handlers.ForgotPasswordHandler)
	mux.HandleFunc("/reset-password", handlers.ResetPasswordHandler)

	mux.HandleFunc("/finance", handlers.ListFinance)

	mux.HandleFunc("/finance/new", handlers.NewFinance)

	mux.HandleFunc("/finance/create", handlers.CreateFinance)

	mux.HandleFunc("/finance/edit", handlers.EditFinance)

	mux.HandleFunc("/finance/update", handlers.UpdateFinance)

	mux.HandleFunc("/finance/delete", handlers.DeleteFinance)

	mux.HandleFunc("/admin/login", handlers.AdminLogin)
	mux.HandleFunc("/login", handlers.AdminLogin)
	mux.HandleFunc("/admin/forgot-password", handlers.AdminForgotPassword)
	mux.HandleFunc("/admin/reset-password", handlers.AdminResetPassword)
	mux.HandleFunc("/admin/change-password", middleware.RequireAuth(handlers.AdminChangePassword))

	mux.HandleFunc("/healthz", handlers.Health)

	// Service Worker for incoming call notifications and mobile responsiveness
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Service-Worker-Allowed", "/")
		swPath := filepath.Join("static", "sw.js")
		if _, err := os.Stat(swPath); os.IsNotExist(err) {
			swPath = filepath.Join("..", "static", "sw.js")
		}
		http.ServeFile(w, r, swPath)
	})

	fs := http.FileServer(http.Dir("static"))
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", fs),
	)

	uploadDir := utils.GetUploadBaseDir()
	_ = os.MkdirAll(filepath.Join(uploadDir, "posts"), 0755)
	_ = os.MkdirAll(filepath.Join(uploadDir, "profiles"), 0755)
	_ = os.MkdirAll(filepath.Join(uploadDir, "covers"), 0755)
	_ = os.MkdirAll(filepath.Join(uploadDir, "documents"), 0755)
	_ = os.MkdirAll(filepath.Join(uploadDir, "chat"), 0755)

	if uploadDir != "uploads" {
		_ = os.MkdirAll("uploads/posts", 0755)
		_ = os.MkdirAll("uploads/profiles", 0755)
		_ = os.MkdirAll("uploads/covers", 0755)
	}

	uploadsFileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir)))

	serveMedia := func(w http.ResponseWriter, r *http.Request, filePath string, customContentType string) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range, Accept-Encoding, Accept, Content-Type, Origin")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges, Content-Type")
		w.Header().Set("Accept-Ranges", "bytes")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".mp4":
			w.Header().Set("Content-Type", "video/mp4")
		case ".webm":
			w.Header().Set("Content-Type", "video/webm")
		case ".mov":
			w.Header().Set("Content-Type", "video/quicktime")
		case ".mkv":
			w.Header().Set("Content-Type", "video/x-matroska")
		case ".avi":
			w.Header().Set("Content-Type", "video/x-msvideo")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".gif":
			w.Header().Set("Content-Type", "image/gif")
		case ".webp":
			w.Header().Set("Content-Type", "image/webp")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		default:
			if customContentType != "" {
				w.Header().Set("Content-Type", customContentType)
			}
		}

		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, filePath)
	}

	mux.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range, Accept-Encoding, Accept, Content-Type, Origin")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges, Content-Type")
		w.Header().Set("Accept-Ranges", "bytes")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		relPath := strings.TrimPrefix(r.URL.Path, "/uploads/")
		relPath = strings.TrimPrefix(relPath, "/")
		fullPath := filepath.Join(uploadDir, filepath.Clean("/"+relPath))

		// 1. Direct file on disk
		if fi, err := os.Stat(fullPath); err == nil && !fi.IsDir() {
			serveMedia(w, r, fullPath, "")
			return
		}

		// 2. Check local fallback directory if uploadDir is different
		if uploadDir != "uploads" {
			localFallback := filepath.Join("uploads", filepath.Clean("/"+relPath))
			if fi, lErr := os.Stat(localFallback); lErr == nil && !fi.IsDir() {
				serveMedia(w, r, localFallback, "")
				return
			}
		}

		// 3. Check subdirectories if requested without folder prefix (e.g. /uploads/pic.jpg -> /uploads/posts/pic.jpg)
		for _, sub := range []string{"posts", "profiles", "covers", "chat"} {
			subPath := filepath.Join(uploadDir, sub, filepath.Clean("/"+relPath))
			if fi, err := os.Stat(subPath); err == nil && !fi.IsDir() {
				serveMedia(w, r, subPath, "")
				return
			}
		}

		// 4. Check PostgreSQL durable media storage
		if data, contentType, err := repository.GetMediaBlob(relPath); err == nil && len(data) > 0 {
			// Restore file to disk cache
			_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
			_ = os.WriteFile(fullPath, data, 0644)

			serveMedia(w, r, fullPath, contentType)
			return
		}

		// 5. Cloud object storage redirect if configured
		if s3Base := utils.GetObjectStoragePublicBaseURL(); s3Base != "" {
			http.Redirect(w, r, strings.TrimSuffix(s3Base, "/")+"/"+strings.TrimPrefix(relPath, "/"), http.StatusTemporaryRedirect)
			return
		} else if utils.IsObjectStorageEnabled() {
			if presignedURL, pErr := utils.GeneratePresignedGetURL(relPath, 6*time.Hour); pErr == nil && presignedURL != "" {
				http.Redirect(w, r, presignedURL, http.StatusTemporaryRedirect)
				return
			}
		}

		// 6. Graceful fallbacks for missing media
		ext := strings.ToLower(filepath.Ext(relPath))
		isVideo := ext == ".mp4" || ext == ".webm" || ext == ".mov" || ext == ".mkv" || ext == ".avi"

		if strings.HasPrefix(relPath, "profiles/") || strings.Contains(relPath, "profile") {
			w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(utils.DefaultAvatarSVG)
			return
		}

		if strings.HasPrefix(relPath, "posts/") && !isVideo {
			w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(utils.DefaultPostImageSVG)
			return
		}

		if isVideo {
			http.Error(w, "Video media file not found", http.StatusNotFound)
			return
		}

		uploadsFileServer.ServeHTTP(w, r)
	})
}

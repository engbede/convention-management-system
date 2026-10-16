package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/utils"
)

// ProgrammesListViewData models template context for /programmes
type ProgrammesListViewData struct {
	LiveProgrammes      []models.Programme
	UpcomingProgrammes  []models.Programme
	PastProgrammes      []models.Programme
	AllProgrammes       []models.Programme
	CurrentTab          string
	CurrentCategory     string
	CurrentUser         *models.User
	IsLoggedIn          bool
	IsAdmin             bool
	UnreadMessagesCount int
	UnreadCount         int
	UnreadNotifications int
}

// ProgrammeViewData models template context for /programmes/view
type ProgrammeViewData struct {
	Programme           *models.Programme
	ChatMessages        []models.ProgrammeChatMessage
	PrayerRequests      []models.ProgrammePrayerRequest
	Moderators          []models.ProgrammeModerator
	CurrentUser         *models.User
	IsLoggedIn          bool
	IsAdmin             bool
	CanManage           bool
	IsModerator         bool
	UnreadMessagesCount int
	UnreadCount         int
	UnreadNotifications int
	ErrorMessage        string
	SuccessMessage      string
}

// ProgrammeFormData models template context for /programmes/new and /programmes/edit
type ProgrammeFormData struct {
	Programme           *models.Programme
	IsEdit              bool
	CurrentUser         *models.User
	IsLoggedIn          bool
	IsAdmin             bool
	UnreadMessagesCount int
	UnreadCount         int
	UnreadNotifications int
	ErrorMessage        string
}

func isCurrentUserAdmin(r *http.Request) bool {
	cookie, err := r.Cookie("admin_session")
	if err == nil {
		if id, err := strconv.Atoi(cookie.Value); err == nil && id > 0 {
			return true
		}
	}
	return false
}

func getAuthenticatedUser(r *http.Request) (*models.User, bool, bool) {
	currentUserID := getCurrentUserID(r)
	isAdmin := isCurrentUserAdmin(r)

	if currentUserID > 0 {
		u, err := repository.GetUserByID(currentUserID)
		if err == nil && u.ID > 0 {
			u.Initials = utils.Initials(u.FullName)
			if u.Role == "admin" {
				isAdmin = true
			}
			return &u, true, isAdmin
		}
	}

	return nil, false, isAdmin
}

func parseFormDateTime(val string) (time.Time, error) {
	clean := strings.TrimSpace(val)
	layouts := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, clean); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date-time format: %s", val)
}

// ListProgrammes renders the live streaming & online programmes index page.
func ListProgrammes(w http.ResponseWriter, r *http.Request) {
	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "live"
	}
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "all"
	}

	liveList, _ := repository.GetLiveProgrammes()
	upcomingList, _ := repository.GetUpcomingProgrammes(20)
	pastList, _ := repository.GetPastProgrammes(20)
	allList, _ := repository.GetAllProgrammes("all", category)

	// Filter by category if specified
	if category != "all" {
		var filteredLive, filteredUpcoming, filteredPast []models.Programme
		for _, p := range liveList {
			if p.Category == category {
				filteredLive = append(filteredLive, p)
			}
		}
		for _, p := range upcomingList {
			if p.Category == category {
				filteredUpcoming = append(filteredUpcoming, p)
			}
		}
		for _, p := range pastList {
			if p.Category == category {
				filteredPast = append(filteredPast, p)
			}
		}
		liveList = filteredLive
		upcomingList = filteredUpcoming
		pastList = filteredPast
	}

	var unreadMsgs, unreadNotifs int
	if currentUser != nil {
		unreadMsgs, _ = repository.CountTotalUnreadMessages(currentUser.ID)
		unreadNotifs, _ = repository.CountUnreadNotifications(currentUser.ID)
	}

	data := ProgrammesListViewData{
		LiveProgrammes:      liveList,
		UpcomingProgrammes:  upcomingList,
		PastProgrammes:      pastList,
		AllProgrammes:       allList,
		CurrentTab:          tab,
		CurrentCategory:     category,
		CurrentUser:         currentUser,
		IsLoggedIn:          isLoggedIn,
		IsAdmin:             isAdmin,
		UnreadMessagesCount: unreadMsgs,
		UnreadCount:         unreadNotifs,
		UnreadNotifications: unreadNotifs,
	}

	Render(w, "programmes_list.html", data)
}

// ViewProgramme renders the single programme view page with live player, chat, and prayer requests.
func ViewProgramme(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	progID, err := strconv.Atoi(idStr)
	if err != nil || progID <= 0 {
		http.Redirect(w, r, "/programmes", http.StatusSeeOther)
		return
	}

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		http.Error(w, "Programme not found", http.StatusNotFound)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)

	// Check access permissions
	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}

	if !services.CanUserViewProgramme(p, currentUserID, isAdmin) {
		// If members only and user not logged in, prompt or redirect to login
		loginRedirect := fmt.Sprintf("/community/login?next=/programmes/view?id=%d", p.ID)
		http.Redirect(w, r, loginRedirect, http.StatusSeeOther)
		return
	}

	canManage := services.CanUserManageProgramme(p, currentUserID, isAdmin)
	isModerator := services.CanUserModerateProgramme(p, currentUserID, isAdmin)
	p.CanManage = canManage
	p.IsModerator = isModerator

	// Fetch chat messages (include hidden if moderator/admin)
	chatMessages, _ := repository.GetProgrammeChatMessages(p.ID, 0, isModerator)

	// Fetch prayer requests (respect privacy!)
	prayerRequests, _ := repository.GetProgrammePrayerRequests(p.ID, currentUserID, isModerator)
	for i := range prayerRequests {
		prayerRequests[i].CanManageStatus = isModerator
	}

	moderators, _ := repository.GetProgrammeModerators(p.ID)

	var unreadMsgs, unreadNotifs int
	if currentUser != nil {
		unreadMsgs, _ = repository.CountTotalUnreadMessages(currentUser.ID)
		unreadNotifs, _ = repository.CountUnreadNotifications(currentUser.ID)
	}

	errMsg := r.URL.Query().Get("error")
	succMsg := r.URL.Query().Get("success")

	data := ProgrammeViewData{
		Programme:           p,
		ChatMessages:        chatMessages,
		PrayerRequests:      prayerRequests,
		Moderators:          moderators,
		CurrentUser:         currentUser,
		IsLoggedIn:          isLoggedIn,
		IsAdmin:             isAdmin,
		CanManage:           canManage,
		IsModerator:         isModerator,
		UnreadMessagesCount: unreadMsgs,
		UnreadCount:         unreadNotifs,
		UnreadNotifications: unreadNotifs,
		ErrorMessage:        errMsg,
		SuccessMessage:      succMsg,
	}

	Render(w, "programme_view.html", data)
}

// NewProgramme displays the programme creation form.
func NewProgramme(w http.ResponseWriter, r *http.Request) {
	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Redirect(w, r, "/community/login?next=/programmes/new", http.StatusSeeOther)
		return
	}

	now := time.Now()
	defaultProg := &models.Programme{
		ScheduleStartTime: now.Add(1 * time.Hour),
		ScheduleEndTime:   now.Add(3 * time.Hour),
		Timezone:          "UTC",
		Visibility:        models.ProgrammeVisibilityPublic,
		Category:          models.ProgrammeCategoryService,
		Status:            models.ProgrammeStatusDraft,
	}

	var unreadMsgs, unreadNotifs int
	if currentUser != nil {
		unreadMsgs, _ = repository.CountTotalUnreadMessages(currentUser.ID)
		unreadNotifs, _ = repository.CountUnreadNotifications(currentUser.ID)
	}

	data := ProgrammeFormData{
		Programme:           defaultProg,
		IsEdit:              false,
		CurrentUser:         currentUser,
		IsLoggedIn:          isLoggedIn,
		IsAdmin:             isAdmin,
		UnreadMessagesCount: unreadMsgs,
		UnreadCount:         unreadNotifs,
		UnreadNotifications: unreadNotifs,
	}

	Render(w, "programme_form.html", data)
}

// CreateProgramme handles form submission for new programme creation.
func CreateProgramme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/programmes/new", http.StatusSeeOther)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	category := strings.TrimSpace(r.FormValue("category"))
	visibility := strings.TrimSpace(r.FormValue("visibility"))
	timezone := strings.TrimSpace(r.FormValue("timezone"))
	ytInput := strings.TrimSpace(r.FormValue("youtube_url"))
	status := strings.TrimSpace(r.FormValue("status"))
	startStr := strings.TrimSpace(r.FormValue("schedule_start_time"))
	endStr := strings.TrimSpace(r.FormValue("schedule_end_time"))

	if title == "" {
		http.Redirect(w, r, "/programmes/new?error=Title+is+required", http.StatusSeeOther)
		return
	}

	var ytID string
	if ytInput != "" {
		extracted, err := services.ExtractYouTubeVideoID(ytInput)
		if err != nil {
			http.Redirect(w, r, "/programmes/new?error=Invalid+YouTube+URL+or+Video+ID", http.StatusSeeOther)
			return
		}
		ytID = extracted
	}

	startTime, err := parseFormDateTime(startStr)
	if err != nil {
		startTime = time.Now().Add(1 * time.Hour)
	}
	endTime, err := parseFormDateTime(endStr)
	if err != nil {
		endTime = startTime.Add(2 * time.Hour)
	}

	if timezone == "" {
		timezone = "UTC"
	}
	if category == "" {
		category = models.ProgrammeCategoryService
	}
	if visibility == "" {
		visibility = models.ProgrammeVisibilityPublic
	}
	if status == "" {
		status = models.ProgrammeStatusDraft
	}

	organizerID := 1
	if currentUser != nil {
		organizerID = currentUser.ID
	}

	p := &models.Programme{
		Title:              title,
		Description:        description,
		Category:           category,
		OrganizerID:        organizerID,
		ScheduleStartTime:  startTime,
		ScheduleEndTime:    endTime,
		Timezone:           timezone,
		Visibility:         visibility,
		YouTubeVideoID:     ytID,
		Status:             status,
		RecordingAvailable: false,
	}

	createdID, err := repository.CreateProgramme(p)
	if err != nil {
		http.Error(w, "Failed to create programme: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programmes/view?id=%d&success=Programme+created+successfully", createdID), http.StatusSeeOther)
}

// EditProgramme displays the programme edit form.
func EditProgramme(w http.ResponseWriter, r *http.Request) {
	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	idStr := r.URL.Query().Get("id")
	progID, err := strconv.Atoi(idStr)
	if err != nil || progID <= 0 {
		http.Redirect(w, r, "/programmes", http.StatusSeeOther)
		return
	}

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		http.Error(w, "Programme not found", http.StatusNotFound)
		return
	}

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}
	if !services.CanUserManageProgramme(p, currentUserID, isAdmin) {
		http.Error(w, "Unauthorized to edit this programme", http.StatusForbidden)
		return
	}

	var unreadMsgs, unreadNotifs int
	if currentUser != nil {
		unreadMsgs, _ = repository.CountTotalUnreadMessages(currentUser.ID)
		unreadNotifs, _ = repository.CountUnreadNotifications(currentUser.ID)
	}

	data := ProgrammeFormData{
		Programme:           p,
		IsEdit:              true,
		CurrentUser:         currentUser,
		IsLoggedIn:          isLoggedIn,
		IsAdmin:             isAdmin,
		UnreadMessagesCount: unreadMsgs,
		UnreadCount:         unreadNotifs,
		UnreadNotifications: unreadNotifs,
		ErrorMessage:        r.URL.Query().Get("error"),
	}

	Render(w, "programme_form.html", data)
}

// UpdateProgramme handles submission of edited programme data.
func UpdateProgramme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/programmes", http.StatusSeeOther)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	progID, err := strconv.Atoi(idStr)
	if err != nil || progID <= 0 {
		http.Redirect(w, r, "/programmes", http.StatusSeeOther)
		return
	}

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		http.Error(w, "Programme not found", http.StatusNotFound)
		return
	}

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}
	if !services.CanUserManageProgramme(p, currentUserID, isAdmin) {
		http.Error(w, "Unauthorized to edit this programme", http.StatusForbidden)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Redirect(w, r, fmt.Sprintf("/programmes/edit?id=%d&error=Title+is+required", progID), http.StatusSeeOther)
		return
	}

	ytInput := strings.TrimSpace(r.FormValue("youtube_url"))
	var ytID string
	if ytInput != "" {
		extracted, err := services.ExtractYouTubeVideoID(ytInput)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/programmes/edit?id=%d&error=Invalid+YouTube+URL+or+Video+ID", progID), http.StatusSeeOther)
			return
		}
		ytID = extracted
	}

	recAvailable := r.FormValue("recording_available") == "1" || r.FormValue("recording_available") == "true"
	recVideoInput := strings.TrimSpace(r.FormValue("recording_video_id"))
	var recVideoID string
	if recVideoInput != "" {
		if recExt, err := services.ExtractYouTubeVideoID(recVideoInput); err == nil {
			recVideoID = recExt
		}
	}

	startStr := strings.TrimSpace(r.FormValue("schedule_start_time"))
	endStr := strings.TrimSpace(r.FormValue("schedule_end_time"))
	startTime, err := parseFormDateTime(startStr)
	if err != nil {
		startTime = p.ScheduleStartTime
	}
	endTime, err := parseFormDateTime(endStr)
	if err != nil {
		endTime = p.ScheduleEndTime
	}

	p.Title = title
	p.Description = strings.TrimSpace(r.FormValue("description"))
	p.Category = strings.TrimSpace(r.FormValue("category"))
	p.Visibility = strings.TrimSpace(r.FormValue("visibility"))
	p.Timezone = strings.TrimSpace(r.FormValue("timezone"))
	p.YouTubeVideoID = ytID
	p.ScheduleStartTime = startTime
	p.ScheduleEndTime = endTime
	p.RecordingAvailable = recAvailable
	p.RecordingVideoID = recVideoID

	if err := repository.UpdateProgramme(p); err != nil {
		http.Error(w, "Failed to update programme: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programmes/view?id=%d&success=Programme+updated+successfully", progID), http.StatusSeeOther)
}

// TransitionProgrammeStatusHandler handles status changes (e.g. go live, end stream, cancel).
func TransitionProgrammeStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	idStr := r.FormValue("id")
	progID, err := strconv.Atoi(idStr)
	if err != nil || progID <= 0 {
		http.Error(w, "Invalid programme ID", http.StatusBadRequest)
		return
	}

	targetStatus := strings.TrimSpace(r.FormValue("status"))
	reason := strings.TrimSpace(r.FormValue("reason"))

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}

	if err := services.TransitionProgrammeStatus(progID, targetStatus, reason, currentUserID, isAdmin); err != nil {
		if strings.HasPrefix(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/programmes/view?id=%d&error=%s", progID, urlQueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	if strings.HasPrefix(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "status": targetStatus})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programmes/view?id=%d&success=Programme+status+updated+to+%s", progID, targetStatus), http.StatusSeeOther)
}

// SendProgrammeChatMessage handles sending a chat message during a programme.
func SendProgrammeChatMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Please log in to participate in the live chat."})
		return
	}

	idStr := r.FormValue("programme_id")
	progID, err := strconv.Atoi(idStr)
	if err != nil || progID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid programme ID"})
		return
	}

	messageText := r.FormValue("message")

	userID := 1
	if currentUser != nil {
		userID = currentUser.ID
	}

	msg, err := services.SendProgrammeChatMessage(progID, userID, messageText)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": msg,
	})
}

// GetProgrammeChatMessagesAPI returns chat messages since a given ID.
func GetProgrammeChatMessagesAPI(w http.ResponseWriter, r *http.Request) {
	progID, _ := strconv.Atoi(r.URL.Query().Get("programme_id"))
	sinceID, _ := strconv.Atoi(r.URL.Query().Get("since"))

	currentUser, _, isAdmin := getAuthenticatedUser(r)
	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Programme not found"})
		return
	}

	// Verify viewing permission
	if !services.CanUserViewProgramme(p, currentUserID, isAdmin) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Access denied"})
		return
	}

	isMod := services.CanUserModerateProgramme(p, currentUserID, isAdmin)
	messages, err := repository.GetProgrammeChatMessages(progID, sinceID, isMod)
	if err != nil {
		messages = []models.ProgrammeChatMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"messages": messages,
	})
}

// HideProgrammeChatMessage handles hiding/unhiding a chat message.
func HideProgrammeChatMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	msgID, _ := strconv.Atoi(r.FormValue("message_id"))
	progID, _ := strconv.Atoi(r.FormValue("programme_id"))
	hidden := r.FormValue("hidden") == "1" || r.FormValue("hidden") == "true"

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Programme not found"})
		return
	}

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}
	if !services.CanUserModerateProgramme(p, currentUserID, isAdmin) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Permission denied"})
		return
	}

	if err := repository.HideProgrammeChatMessage(msgID, hidden); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// SubmitPrayerRequest handles prayer request submissions.
func SubmitPrayerRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Please log in to submit a prayer request."})
		return
	}

	progID, _ := strconv.Atoi(r.FormValue("programme_id"))
	requestText := r.FormValue("request")
	isPrivate := r.FormValue("is_private") == "1" || r.FormValue("is_private") == "true"

	userID := 1
	if currentUser != nil {
		userID = currentUser.ID
	}

	req, err := services.SubmitProgrammePrayerRequest(progID, userID, requestText, isPrivate)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"prayer_request": req,
	})
}

// GetProgrammePrayerRequestsAPI returns prayer requests respecting privacy.
func GetProgrammePrayerRequestsAPI(w http.ResponseWriter, r *http.Request) {
	progID, _ := strconv.Atoi(r.URL.Query().Get("programme_id"))

	currentUser, _, isAdmin := getAuthenticatedUser(r)
	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Programme not found"})
		return
	}

	if !services.CanUserViewProgramme(p, currentUserID, isAdmin) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Access denied"})
		return
	}

	isMod := services.CanUserModerateProgramme(p, currentUserID, isAdmin)
	requests, err := repository.GetProgrammePrayerRequests(progID, currentUserID, isMod)
	if err != nil {
		requests = []models.ProgrammePrayerRequest{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"requests": requests,
	})
}

// UpdatePrayerRequestStatusHandler handles status update of prayer requests.
func UpdatePrayerRequestStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	reqID, _ := strconv.Atoi(r.FormValue("request_id"))
	progID, _ := strconv.Atoi(r.FormValue("programme_id"))
	status := strings.TrimSpace(r.FormValue("status"))

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Programme not found"})
		return
	}

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}
	if !services.CanUserModerateProgramme(p, currentUserID, isAdmin) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Permission denied"})
		return
	}

	if err := repository.UpdatePrayerRequestStatus(reqID, status); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "status": status})
}

// AssignModeratorHandler assigns a moderator to a programme.
func AssignModeratorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	progID, _ := strconv.Atoi(r.FormValue("programme_id"))
	targetUserID, _ := strconv.Atoi(r.FormValue("user_id"))

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		http.Error(w, "Programme not found", http.StatusNotFound)
		return
	}

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}
	if !services.CanUserManageProgramme(p, currentUserID, isAdmin) {
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	_ = repository.AddProgrammeModerator(progID, targetUserID)
	http.Redirect(w, r, fmt.Sprintf("/programmes/view?id=%d&success=Moderator+assigned", progID), http.StatusSeeOther)
}

// RemoveModeratorHandler removes a moderator from a programme.
func RemoveModeratorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, isLoggedIn, isAdmin := getAuthenticatedUser(r)
	if !isLoggedIn && !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	progID, _ := strconv.Atoi(r.FormValue("programme_id"))
	targetUserID, _ := strconv.Atoi(r.FormValue("user_id"))

	p, err := repository.GetProgrammeByID(progID)
	if err != nil || p == nil {
		http.Error(w, "Programme not found", http.StatusNotFound)
		return
	}

	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}
	if !services.CanUserManageProgramme(p, currentUserID, isAdmin) {
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	_ = repository.RemoveProgrammeModerator(progID, targetUserID)
	http.Redirect(w, r, fmt.Sprintf("/programmes/view?id=%d&success=Moderator+removed", progID), http.StatusSeeOther)
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}

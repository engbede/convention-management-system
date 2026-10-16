package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
)

// scanProgrammeRow scans a single programme row from a database scanner.
func scanProgrammeRow(scanner interface{ Scan(dest ...any) error }) (*models.Programme, error) {
	var p models.Programme
	var (
		descNull    sql.NullString
		ytIDNull    sql.NullString
		reasonNull  sql.NullString
		recVidNull  sql.NullString
		startRaw    any
		endRaw      any
		createdRaw  any
		updatedRaw  any
		recAvailInt int
	)

	err := scanner.Scan(
		&p.ID,
		&p.Title,
		&descNull,
		&p.Category,
		&p.OrganizerID,
		&startRaw,
		&endRaw,
		&p.Timezone,
		&p.Visibility,
		&ytIDNull,
		&p.Status,
		&reasonNull,
		&recAvailInt,
		&recVidNull,
		&createdRaw,
		&updatedRaw,
	)
	if err != nil {
		return nil, err
	}

	p.Description = descNull.String
	p.YouTubeVideoID = ytIDNull.String
	p.CancellationReason = reasonNull.String
	p.RecordingVideoID = recVidNull.String
	p.RecordingAvailable = recAvailInt == 1
	p.ScheduleStartTime = parseSQLTime(startRaw)
	p.ScheduleEndTime = parseSQLTime(endRaw)
	p.CreatedAt = parseSQLTime(createdRaw)
	p.UpdatedAt = parseSQLTime(updatedRaw)

	return &p, nil
}

// populateProgrammeOrganizer attaches the user model for the organizer.
func populateProgrammeOrganizer(p *models.Programme) {
	if p == nil || p.OrganizerID == 0 || database.DB == nil {
		return
	}
	org, err := GetUserByID(p.OrganizerID)
	if err == nil && org.ID > 0 {
		p.Organizer = &org
	}
}

// CreateProgramme persists a new live programme in the database.
func CreateProgramme(p *models.Programme) (int, error) {
	if database.DB == nil {
		return 0, nil
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now()
	}
	if p.Status == "" {
		p.Status = models.ProgrammeStatusDraft
	}
	if p.Visibility == "" {
		p.Visibility = models.ProgrammeVisibilityPublic
	}
	if p.Category == "" {
		p.Category = models.ProgrammeCategoryService
	}
	if p.Timezone == "" {
		p.Timezone = "UTC"
	}

	recAvail := 0
	if p.RecordingAvailable {
		recAvail = 1
	}

	query := fmt.Sprintf(`
		INSERT INTO programmes (
			title, description, category, organizer_id,
			schedule_start_time, schedule_end_time, timezone,
			visibility, youtube_video_id, status, cancellation_reason,
			recording_available, recording_video_id, created_at, updated_at
		)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
	`,
		helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4),
		helpers.Placeholder(5), helpers.Placeholder(6), helpers.Placeholder(7), helpers.Placeholder(8),
		helpers.Placeholder(9), helpers.Placeholder(10), helpers.Placeholder(11), helpers.Placeholder(12),
		helpers.Placeholder(13), helpers.Placeholder(14), helpers.Placeholder(15),
	)

	result, err := database.DB.Exec(
		query,
		p.Title,
		p.Description,
		p.Category,
		p.OrganizerID,
		p.ScheduleStartTime.Format("2006-01-02 15:04:05"),
		p.ScheduleEndTime.Format("2006-01-02 15:04:05"),
		p.Timezone,
		p.Visibility,
		p.YouTubeVideoID,
		p.Status,
		p.CancellationReason,
		recAvail,
		p.RecordingVideoID,
		p.CreatedAt.Format("2006-01-02 15:04:05"),
		p.UpdatedAt.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil || id == 0 {
		var lastID int
		err = database.DB.QueryRow("SELECT id FROM programmes WHERE organizer_id = ? ORDER BY id DESC LIMIT 1", p.OrganizerID).Scan(&lastID)
		if err != nil {
			return 0, err
		}
		p.ID = lastID
		return lastID, nil
	}

	p.ID = int(id)
	return int(id), nil
}

// UpdateProgramme updates an existing programme's basic metadata.
func UpdateProgramme(p *models.Programme) error {
	if database.DB == nil {
		return nil
	}
	p.UpdatedAt = time.Now()
	recAvail := 0
	if p.RecordingAvailable {
		recAvail = 1
	}

	query := fmt.Sprintf(`
		UPDATE programmes
		SET title = %s, description = %s, category = %s,
		    schedule_start_time = %s, schedule_end_time = %s, timezone = %s,
		    visibility = %s, youtube_video_id = %s, recording_available = %s,
		    recording_video_id = %s, updated_at = %s
		WHERE id = %s
	`,
		helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3),
		helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6),
		helpers.Placeholder(7), helpers.Placeholder(8), helpers.Placeholder(9),
		helpers.Placeholder(10), helpers.Placeholder(11), helpers.Placeholder(12),
	)

	_, err := database.DB.Exec(
		query,
		p.Title,
		p.Description,
		p.Category,
		p.ScheduleStartTime.Format("2006-01-02 15:04:05"),
		p.ScheduleEndTime.Format("2006-01-02 15:04:05"),
		p.Timezone,
		p.Visibility,
		p.YouTubeVideoID,
		recAvail,
		p.RecordingVideoID,
		p.UpdatedAt.Format("2006-01-02 15:04:05"),
		p.ID,
	)
	return err
}

// GetProgrammeByID retrieves a single programme by ID.
func GetProgrammeByID(id int) (*models.Programme, error) {
	if database.DB == nil {
		return nil, sql.ErrNoRows
	}
	query := fmt.Sprintf(`
		SELECT id, title, description, category, organizer_id,
		       schedule_start_time, schedule_end_time, timezone,
		       visibility, youtube_video_id, status, cancellation_reason,
		       recording_available, recording_video_id, created_at, updated_at
		FROM programmes
		WHERE id = %s
		LIMIT 1
	`, helpers.Placeholder(1))

	row := database.DB.QueryRow(query, id)
	p, err := scanProgrammeRow(row)
	if err != nil {
		return nil, err
	}
	populateProgrammeOrganizer(p)
	return p, nil
}

// UpdateProgrammeStatus updates the status of a programme and optionally its cancellation reason.
func UpdateProgrammeStatus(id int, newStatus string, cancellationReason string) error {
	if database.DB == nil {
		return nil
	}
	query := fmt.Sprintf(`
		UPDATE programmes
		SET status = %s, cancellation_reason = %s, updated_at = %s
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))

	_, err := database.DB.Exec(query, newStatus, cancellationReason, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

// UpdateProgrammeRecording updates recording flags and ID.
func UpdateProgrammeRecording(id int, recordingAvailable bool, recordingVideoID string) error {
	if database.DB == nil {
		return nil
	}
	recAvail := 0
	if recordingAvailable {
		recAvail = 1
	}

	query := fmt.Sprintf(`
		UPDATE programmes
		SET recording_available = %s, recording_video_id = %s, updated_at = %s
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))

	_, err := database.DB.Exec(query, recAvail, recordingVideoID, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

// DeleteProgramme removes a programme by ID.
func DeleteProgramme(id int) error {
	if database.DB == nil {
		return nil
	}
	query := fmt.Sprintf("DELETE FROM programmes WHERE id = %s", helpers.Placeholder(1))
	_, err := database.DB.Exec(query, id)
	return err
}

// GetLiveProgrammes returns all currently live programmes.
func GetLiveProgrammes() ([]models.Programme, error) {
	if database.DB == nil {
		return nil, nil
	}
	query := fmt.Sprintf(`
		SELECT id, title, description, category, organizer_id,
		       schedule_start_time, schedule_end_time, timezone,
		       visibility, youtube_video_id, status, cancellation_reason,
		       recording_available, recording_video_id, created_at, updated_at
		FROM programmes
		WHERE status = %s
		ORDER BY schedule_start_time ASC
	`, helpers.Placeholder(1))

	rows, err := database.DB.Query(query, models.ProgrammeStatusLive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Programme
	for rows.Next() {
		p, err := scanProgrammeRow(rows)
		if err == nil && p != nil {
			populateProgrammeOrganizer(p)
			list = append(list, *p)
		}
	}
	return list, nil
}

// GetUpcomingProgrammes returns scheduled upcoming programmes.
func GetUpcomingProgrammes(limit int) ([]models.Programme, error) {
	if database.DB == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	query := fmt.Sprintf(`
		SELECT id, title, description, category, organizer_id,
		       schedule_start_time, schedule_end_time, timezone,
		       visibility, youtube_video_id, status, cancellation_reason,
		       recording_available, recording_video_id, created_at, updated_at
		FROM programmes
		WHERE status = %s
		ORDER BY schedule_start_time ASC
		LIMIT %d
	`, helpers.Placeholder(1), limit)

	rows, err := database.DB.Query(query, models.ProgrammeStatusScheduled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Programme
	for rows.Next() {
		p, err := scanProgrammeRow(rows)
		if err == nil && p != nil {
			populateProgrammeOrganizer(p)
			list = append(list, *p)
		}
	}
	return list, nil
}

// GetPastProgrammes returns ended programmes (recordings or archives).
func GetPastProgrammes(limit int) ([]models.Programme, error) {
	if database.DB == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	query := fmt.Sprintf(`
		SELECT id, title, description, category, organizer_id,
		       schedule_start_time, schedule_end_time, timezone,
		       visibility, youtube_video_id, status, cancellation_reason,
		       recording_available, recording_video_id, created_at, updated_at
		FROM programmes
		WHERE status = %s
		ORDER BY schedule_end_time DESC
		LIMIT %d
	`, helpers.Placeholder(1), limit)

	rows, err := database.DB.Query(query, models.ProgrammeStatusEnded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Programme
	for rows.Next() {
		p, err := scanProgrammeRow(rows)
		if err == nil && p != nil {
			populateProgrammeOrganizer(p)
			list = append(list, *p)
		}
	}
	return list, nil
}

// GetAllProgrammes retrieves programmes with optional filtering by status and category.
func GetAllProgrammes(statusFilter, categoryFilter string) ([]models.Programme, error) {
	if database.DB == nil {
		return nil, nil
	}
	var whereClauses []string
	var args []any
	argIdx := 1

	if statusFilter != "" && statusFilter != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = %s", helpers.Placeholder(argIdx)))
		args = append(args, statusFilter)
		argIdx++
	}

	if categoryFilter != "" && categoryFilter != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("category = %s", helpers.Placeholder(argIdx)))
		args = append(args, categoryFilter)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, title, description, category, organizer_id,
		       schedule_start_time, schedule_end_time, timezone,
		       visibility, youtube_video_id, status, cancellation_reason,
		       recording_available, recording_video_id, created_at, updated_at
		FROM programmes
		%s
		ORDER BY 
			CASE status
				WHEN 'live' THEN 1
				WHEN 'scheduled' THEN 2
				WHEN 'draft' THEN 3
				WHEN 'ended' THEN 4
				ELSE 5
			END,
			schedule_start_time ASC
	`, whereSQL)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Programme
	for rows.Next() {
		p, err := scanProgrammeRow(rows)
		if err == nil && p != nil {
			populateProgrammeOrganizer(p)
			list = append(list, *p)
		}
	}
	return list, nil
}

// ==========================================
// Moderator Management
// ==========================================

// AddProgrammeModerator assigns a user as moderator for a programme.
func AddProgrammeModerator(programmeID, userID int) error {
	if database.DB == nil {
		return nil
	}
	query := fmt.Sprintf(`
		INSERT INTO programme_moderators (programme_id, user_id, assigned_at)
		VALUES (%s, %s, %s)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err := database.DB.Exec(query, programmeID, userID, time.Now().Format("2006-01-02 15:04:05"))
	return err
}

// RemoveProgrammeModerator removes a moderator assignment.
func RemoveProgrammeModerator(programmeID, userID int) error {
	if database.DB == nil {
		return nil
	}
	query := fmt.Sprintf(`
		DELETE FROM programme_moderators
		WHERE programme_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	_, err := database.DB.Exec(query, programmeID, userID)
	return err
}

// GetProgrammeModerators lists all moderators for a programme with user profiles.
func GetProgrammeModerators(programmeID int) ([]models.ProgrammeModerator, error) {
	if database.DB == nil {
		return nil, nil
	}
	query := fmt.Sprintf(`
		SELECT pm.id, pm.programme_id, pm.user_id, pm.assigned_at,
		       u.id, u.full_name, u.username, u.profile_photo, u.role
		FROM programme_moderators pm
		JOIN users u ON pm.user_id = u.id
		WHERE pm.programme_id = %s
		ORDER BY pm.assigned_at ASC
	`, helpers.Placeholder(1))

	rows, err := database.DB.Query(query, programmeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ProgrammeModerator
	for rows.Next() {
		var mod models.ProgrammeModerator
		var u models.User
		var (
			assignedRaw any
			photoNull   sql.NullString
			roleNull    sql.NullString
		)

		err := rows.Scan(
			&mod.ID,
			&mod.ProgrammeID,
			&mod.UserID,
			&assignedRaw,
			&u.ID,
			&u.FullName,
			&u.Username,
			&photoNull,
			&roleNull,
		)
		if err == nil {
			u.ProfilePhoto = photoNull.String
			u.Role = roleNull.String
			mod.User = &u
			mod.AssignedAt = parseSQLTime(assignedRaw)
			list = append(list, mod)
		}
	}
	return list, nil
}

// IsProgrammeModerator checks whether a user is an assigned moderator for a programme.
func IsProgrammeModerator(programmeID, userID int) (bool, error) {
	if database.DB == nil {
		return false, nil
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM programme_moderators
		WHERE programme_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var count int
	err := database.DB.QueryRow(query, programmeID, userID).Scan(&count)
	return count > 0, err
}

// ==========================================
// Live Chat Messages
// ==========================================

// CreateProgrammeChatMessage inserts a chat message.
func CreateProgrammeChatMessage(msg *models.ProgrammeChatMessage) error {
	if database.DB == nil {
		return nil
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	hiddenInt := 0
	if msg.IsHidden {
		hiddenInt = 1
	}

	query := fmt.Sprintf(`
		INSERT INTO programme_chat_messages (programme_id, user_id, message, is_hidden, created_at)
		VALUES (%s, %s, %s, %s, %s)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))

	result, err := database.DB.Exec(
		query,
		msg.ProgrammeID,
		msg.UserID,
		msg.Message,
		hiddenInt,
		msg.CreatedAt.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		msg.ID = int(id)
	}
	return nil
}

// GetProgrammeChatMessages returns chat messages for a programme.
// If includeHidden is false, hidden messages are filtered out.
func GetProgrammeChatMessages(programmeID int, sinceID int, includeHidden bool) ([]models.ProgrammeChatMessage, error) {
	if database.DB == nil {
		return nil, nil
	}
	var whereParts []string
	var args []any
	argIdx := 1

	whereParts = append(whereParts, fmt.Sprintf("pcm.programme_id = %s", helpers.Placeholder(argIdx)))
	args = append(args, programmeID)
	argIdx++

	if sinceID > 0 {
		whereParts = append(whereParts, fmt.Sprintf("pcm.id > %s", helpers.Placeholder(argIdx)))
		args = append(args, sinceID)
		argIdx++
	}

	if !includeHidden {
		whereParts = append(whereParts, "pcm.is_hidden = 0")
	}

	query := fmt.Sprintf(`
		SELECT pcm.id, pcm.programme_id, pcm.user_id, pcm.message, pcm.is_hidden, pcm.created_at,
		       u.id, u.full_name, u.username, u.profile_photo, u.role
		FROM programme_chat_messages pcm
		JOIN users u ON pcm.user_id = u.id
		WHERE %s
		ORDER BY pcm.created_at ASC
		LIMIT 100
	`, strings.Join(whereParts, " AND "))

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ProgrammeChatMessage
	for rows.Next() {
		var msg models.ProgrammeChatMessage
		var u models.User
		var (
			hiddenInt  int
			createdRaw any
			photoNull  sql.NullString
			roleNull   sql.NullString
		)

		err := rows.Scan(
			&msg.ID,
			&msg.ProgrammeID,
			&msg.UserID,
			&msg.Message,
			&hiddenInt,
			&createdRaw,
			&u.ID,
			&u.FullName,
			&u.Username,
			&photoNull,
			&roleNull,
		)
		if err == nil {
			msg.IsHidden = hiddenInt == 1
			msg.CreatedAt = parseSQLTime(createdRaw)
			u.ProfilePhoto = photoNull.String
			u.Role = roleNull.String
			msg.User = &u
			list = append(list, msg)
		}
	}
	return list, nil
}

// HideProgrammeChatMessage toggles whether a chat message is hidden by moderation.
func HideProgrammeChatMessage(messageID int, isHidden bool) error {
	if database.DB == nil {
		return nil
	}
	val := 0
	if isHidden {
		val = 1
	}
	query := fmt.Sprintf("UPDATE programme_chat_messages SET is_hidden = %s WHERE id = %s", helpers.Placeholder(1), helpers.Placeholder(2))
	_, err := database.DB.Exec(query, val, messageID)
	return err
}

// GetLastChatMessageTimeByUser finds the timestamp of the latest chat message from a given user in a programme.
func GetLastChatMessageTimeByUser(programmeID, userID int) (time.Time, error) {
	if database.DB == nil {
		return time.Time{}, nil
	}
	query := fmt.Sprintf(`
		SELECT created_at
		FROM programme_chat_messages
		WHERE programme_id = %s AND user_id = %s
		ORDER BY id DESC
		LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var raw any
	err := database.DB.QueryRow(query, programmeID, userID).Scan(&raw)
	if err != nil {
		return time.Time{}, err
	}
	return parseSQLTime(raw), nil
}

// ==========================================
// Prayer Requests
// ==========================================

// CreateProgrammePrayerRequest inserts a new prayer request.
func CreateProgrammePrayerRequest(req *models.ProgrammePrayerRequest) error {
	if database.DB == nil {
		return nil
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.Status == "" {
		req.Status = models.PrayerStatusPending
	}

	privInt := 0
	if req.IsPrivate {
		privInt = 1
	}

	query := fmt.Sprintf(`
		INSERT INTO programme_prayer_requests (programme_id, user_id, request, is_private, status, created_at)
		VALUES (%s, %s, %s, %s, %s, %s)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6))

	result, err := database.DB.Exec(
		query,
		req.ProgrammeID,
		req.UserID,
		req.Request,
		privInt,
		req.Status,
		req.CreatedAt.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		req.ID = int(id)
	}
	return nil
}

// GetProgrammePrayerRequests retrieves prayer requests for a programme.
// If canViewAllPrivate is true, all prayer requests are returned.
// Otherwise, only public prayer requests or requests owned by viewerUserID are returned.
func GetProgrammePrayerRequests(programmeID int, viewerUserID int, canViewAllPrivate bool) ([]models.ProgrammePrayerRequest, error) {
	if database.DB == nil {
		return nil, nil
	}
	var query string
	var args []any

	if canViewAllPrivate {
		query = fmt.Sprintf(`
			SELECT pr.id, pr.programme_id, pr.user_id, pr.request, pr.is_private, pr.status, pr.created_at,
			       u.id, u.full_name, u.username, u.profile_photo
			FROM programme_prayer_requests pr
			JOIN users u ON pr.user_id = u.id
			WHERE pr.programme_id = %s
			ORDER BY pr.created_at DESC
		`, helpers.Placeholder(1))
		args = []any{programmeID}
	} else {
		query = fmt.Sprintf(`
			SELECT pr.id, pr.programme_id, pr.user_id, pr.request, pr.is_private, pr.status, pr.created_at,
			       u.id, u.full_name, u.username, u.profile_photo
			FROM programme_prayer_requests pr
			JOIN users u ON pr.user_id = u.id
			WHERE pr.programme_id = %s AND (pr.is_private = 0 OR pr.user_id = %s)
			ORDER BY pr.created_at DESC
		`, helpers.Placeholder(1), helpers.Placeholder(2))
		args = []any{programmeID, viewerUserID}
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ProgrammePrayerRequest
	for rows.Next() {
		var req models.ProgrammePrayerRequest
		var u models.User
		var (
			privInt    int
			createdRaw any
			photoNull  sql.NullString
		)

		err := rows.Scan(
			&req.ID,
			&req.ProgrammeID,
			&req.UserID,
			&req.Request,
			&privInt,
			&req.Status,
			&createdRaw,
			&u.ID,
			&u.FullName,
			&u.Username,
			&photoNull,
		)
		if err == nil {
			req.IsPrivate = privInt == 1
			req.CreatedAt = parseSQLTime(createdRaw)
			u.ProfilePhoto = photoNull.String
			req.User = &u
			list = append(list, req)
		}
	}
	return list, nil
}

// GetProgrammePrayerRequestByID retrieves a single prayer request by ID.
func GetProgrammePrayerRequestByID(requestID int) (*models.ProgrammePrayerRequest, error) {
	if database.DB == nil {
		return nil, sql.ErrNoRows
	}
	query := fmt.Sprintf(`
		SELECT id, programme_id, user_id, request, is_private, status, created_at
		FROM programme_prayer_requests
		WHERE id = %s
		LIMIT 1
	`, helpers.Placeholder(1))

	var req models.ProgrammePrayerRequest
	var (
		privInt    int
		createdRaw any
	)

	err := database.DB.QueryRow(query, requestID).Scan(
		&req.ID,
		&req.ProgrammeID,
		&req.UserID,
		&req.Request,
		&privInt,
		&req.Status,
		&createdRaw,
	)
	if err != nil {
		return nil, err
	}
	req.IsPrivate = privInt == 1
	req.CreatedAt = parseSQLTime(createdRaw)
	return &req, nil
}

// UpdatePrayerRequestStatus updates the status of a prayer request (e.g. pending, praying, answered).
func UpdatePrayerRequestStatus(requestID int, status string) error {
	if database.DB == nil {
		return nil
	}
	query := fmt.Sprintf(`
		UPDATE programme_prayer_requests
		SET status = %s
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	_, err := database.DB.Exec(query, status, requestID)
	return err
}

// ==========================================
// Notification Log (Deduplication)
// ==========================================

// HasProgrammeNotificationBeenSent checks if a notification type has already been logged.
func HasProgrammeNotificationBeenSent(programmeID int, notificationType string) (bool, error) {
	if database.DB == nil {
		return false, nil
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM programme_notifications_log
		WHERE programme_id = %s AND notification_type = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var count int
	err := database.DB.QueryRow(query, programmeID, notificationType).Scan(&count)
	return count > 0, err
}

// LogProgrammeNotificationSent records that a notification type was dispatched.
func LogProgrammeNotificationSent(programmeID int, notificationType string) error {
	if database.DB == nil {
		return nil
	}
	query := fmt.Sprintf(`
		INSERT INTO programme_notifications_log (programme_id, notification_type, sent_at)
		VALUES (%s, %s, %s)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err := database.DB.Exec(query, programmeID, notificationType, time.Now().Format("2006-01-02 15:04:05"))
	return err
}

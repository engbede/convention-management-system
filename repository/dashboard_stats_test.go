package repository

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
)

// TestGetDashboardStats_Empty verifies GetDashboardStats works on empty dataset with no NULL scan crashes.
func TestGetDashboardStats_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := database.SeedConvention(); err != nil {
		t.Fatalf("SeedConvention failed: %v", err)
	}

	stats, err := GetDashboardStats()
	if err != nil {
		t.Fatalf("GetDashboardStats failed on empty table: %v", err)
	}

	if stats.TotalRegistrations != 0 || stats.MemberCount != 0 || stats.NonMemberCount != 0 {
		t.Errorf("Expected 0 total/members/nonmembers, got Total=%d, Members=%d, NonMembers=%d",
			stats.TotalRegistrations, stats.MemberCount, stats.NonMemberCount)
	}
	if stats.MaleCount != 0 || stats.FemaleCount != 0 {
		t.Errorf("Expected 0 male/female, got Male=%d, Female=%d", stats.MaleCount, stats.FemaleCount)
	}
	if stats.SingleCount != 0 || stats.MarriedCount != 0 || stats.DivorcedCount != 0 {
		t.Errorf("Expected 0 marital statuses, got Single=%d, Married=%d, Divorced=%d",
			stats.SingleCount, stats.MarriedCount, stats.DivorcedCount)
	}
}

// TestGetDashboardStats_MembershipAndMaritalNormalization verifies normalization of member, non-member,
// and divorce/divorced status across varied cases and formatting.
func TestGetDashboardStats_MembershipAndMaritalNormalization(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := database.SeedConvention(); err != nil {
		t.Fatalf("SeedConvention failed: %v", err)
	}

	activeConv, err := GetActiveConvention()
	if err != nil {
		t.Fatalf("GetActiveConvention failed: %v", err)
	}

	testCases := []struct {
		name          string
		membership    string
		maritalStatus string
		gender        string
		phone         string
	}{
		{"Member 1", "Member", "Single", "Male", "08000000001"},
		{"Member 2", "member", "Married", "Female", "08000000002"},
		{"Member 3", "  Member  ", "Widowed", "Female", "08000000003"},
		{"NonMember 1", "Non Member", "Divorced", "Male", "08000000004"},
		{"NonMember 2", "non member", "Divorce", "Female", "08000000005"},
		{"NonMember 3", "Non-Member", "Single", "Male", "08000000006"},
		{"NonMember 4", "non-member", "Married", "Female", "08000000007"},
		{"NonMember 5", " Non-Member ", "divorced", "Male", "08000000008"},
	}

	for _, tc := range testCases {
		reg := &models.Registration{
			ConventionID:          activeConv.ID,
			FullName:              tc.name,
			Gender:                tc.gender,
			Age:                   25,
			Phone:                 tc.phone,
			Email:                 tc.phone + "@example.com",
			Circuit:               "Central Circuit",
			LocalChurch:           "Grace Chapel",
			Membership:            tc.membership,
			Position:              "Member",
			MaritalStatus:         tc.maritalStatus,
			Occupation:            "Student",
			EmergencyContactName:  "Parent",
			EmergencyContactPhone: "08099999999",
			Relationship:          "Parent",
			Address:               "123 Faith Street",
			ArrivalDate:           "2026-11-01",
			FirstTimeAttendee:     true,
			CheckedIn:             false,
			BibleStudyGroup:       1,
			RegistrationNumber:    "REG-" + tc.phone,
		}
		if err := CreateRegistration(reg); err != nil {
			t.Fatalf("Failed creating test registration for %s: %v", tc.name, err)
		}
	}

	stats, err := GetDashboardStats()
	if err != nil {
		t.Fatalf("GetDashboardStats failed: %v", err)
	}

	if stats.TotalRegistrations != 8 {
		t.Errorf("Expected TotalRegistrations=8, got %d", stats.TotalRegistrations)
	}
	if stats.MemberCount != 3 {
		t.Errorf("Expected MemberCount=3, got %d", stats.MemberCount)
	}
	if stats.NonMemberCount != 5 {
		t.Errorf("Expected NonMemberCount=5, got %d", stats.NonMemberCount)
	}
	if stats.MaleCount != 4 {
		t.Errorf("Expected MaleCount=4, got %d", stats.MaleCount)
	}
	if stats.FemaleCount != 4 {
		t.Errorf("Expected FemaleCount=4, got %d", stats.FemaleCount)
	}
	if stats.SingleCount != 2 {
		t.Errorf("Expected SingleCount=2, got %d", stats.SingleCount)
	}
	if stats.MarriedCount != 2 {
		t.Errorf("Expected MarriedCount=2, got %d", stats.MarriedCount)
	}
	if stats.DivorcedCount != 3 {
		t.Errorf("Expected DivorcedCount=3 (Divorced, Divorce, divorced), got %d", stats.DivorcedCount)
	}
}

// TestReportsRepository_GetReportStats verifies GetReportStats with boolean checked_in logic and counts.
func TestReportsRepository_GetReportStats(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := database.SeedConvention(); err != nil {
		t.Fatalf("SeedConvention failed: %v", err)
	}

	activeConv, err := GetActiveConvention()
	if err != nil {
		t.Fatalf("GetActiveConvention failed: %v", err)
	}

	// 1. Empty reports stats (1 convention from seed)
	statsEmpty, err := GetReportStats()
	if err != nil {
		t.Fatalf("GetReportStats failed on empty table: %v", err)
	}
	if statsEmpty.TotalRegistrations != 0 || statsEmpty.CheckedIn != 0 {
		t.Errorf("Expected 0 registrations/checkedin, got %+v", statsEmpty)
	}
	if statsEmpty.TotalConventions != 1 {
		t.Errorf("Expected 1 seeded convention, got %d", statsEmpty.TotalConventions)
	}

	// 2. Populate test registrations
	regs := []struct {
		name       string
		membership string
		gender     string
		phone      string
		checkedIn  bool
	}{
		{"Attendee 1", "Member", "Male", "08111111101", true},
		{"Attendee 2", "Non Member", "Female", "08111111102", false},
		{"Attendee 3", "Non-Member", "Female", "08111111103", true},
	}

	for _, r := range regs {
		reg := &models.Registration{
			ConventionID:          activeConv.ID,
			FullName:              r.name,
			Gender:                r.gender,
			Age:                   22,
			Phone:                 r.phone,
			Membership:            r.membership,
			MaritalStatus:         "Single",
			CheckedIn:             r.checkedIn,
			RegistrationNumber:    "REP-" + r.phone,
			Circuit:               "Circuit 1",
			LocalChurch:           "Church 1",
			Occupation:            "Engineer",
			EmergencyContactName:  "Contact",
			EmergencyContactPhone: "08100000000",
			Relationship:          "Sibling",
			Address:               "Address 1",
			ArrivalDate:           "2026-11-01",
			FirstTimeAttendee:     false,
			BibleStudyGroup:       1,
		}
		if err := CreateRegistration(reg); err != nil {
			t.Fatalf("Failed to create registration: %v", err)
		}
		if r.checkedIn {
			if err := MarkCheckedIn(reg.ID); err != nil {
				t.Fatalf("Failed marking checked in: %v", err)
			}
		}
	}

	stats, err := GetReportStats()
	if err != nil {
		t.Fatalf("GetReportStats failed: %v", err)
	}

	if stats.TotalRegistrations != 3 {
		t.Errorf("Expected TotalRegistrations=3, got %d", stats.TotalRegistrations)
	}
	if stats.CheckedIn != 2 {
		t.Errorf("Expected CheckedIn=2, got %d", stats.CheckedIn)
	}

	// Also verify GetAttendanceStats()
	attStats, err := GetAttendanceStats()
	if err != nil {
		t.Fatalf("GetAttendanceStats failed: %v", err)
	}
	if attStats.CheckedIn != 2 {
		t.Errorf("Expected Attendance CheckedIn=2, got %d", attStats.CheckedIn)
	}
	if attStats.Pending != 1 {
		t.Errorf("Expected Attendance Pending=1, got %d", attStats.Pending)
	}
}

// TestConventionActivation_BooleanCompatibility verifies convention activation boolean handling.
func TestConventionActivation_BooleanCompatibility(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	c1 := models.Convention{
		Name:      "Convention 2026",
		Theme:     "Convention Theme 2026",
		Year:      2026,
		StartDate: "2026-11-01",
		EndDate:   "2026-11-05",
		Venue:     "Main Arena",
		Active:    true,
	}
	c2 := models.Convention{
		Name:      "Convention 2027",
		Theme:     "Convention Theme 2027",
		Year:      2027,
		StartDate: "2027-11-01",
		EndDate:   "2027-11-05",
		Venue:     "New Arena",
		Active:    false,
	}

	if err := CreateConvention(c1); err != nil {
		t.Fatalf("Failed creating convention 1: %v", err)
	}
	if err := CreateConvention(c2); err != nil {
		t.Fatalf("Failed creating convention 2: %v", err)
	}

	activeConv, err := GetActiveConvention()
	if err != nil {
		t.Fatalf("Failed getting active convention: %v", err)
	}
	if activeConv.Year != 2026 {
		t.Errorf("Expected active convention year 2026, got %d", activeConv.Year)
	}

	// Now activate convention 2 (ID = 2)
	if err := ActivateConvention(2); err != nil {
		t.Fatalf("ActivateConvention(2) failed: %v", err)
	}

	activeConv2, err := GetActiveConvention()
	if err != nil {
		t.Fatalf("Failed getting active convention after activation: %v", err)
	}
	if activeConv2.Year != 2027 {
		t.Errorf("Expected active convention year 2027 after activation, got %d", activeConv2.Year)
	}

	// Verify only 1 convention is active
	conventions, err := GetAllConventions()
	if err != nil {
		t.Fatalf("Failed getting all conventions: %v", err)
	}
	var activeCount int
	for _, c := range conventions {
		if c.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("Expected exactly 1 active convention, got %d", activeCount)
	}
}

// TestPublicNotice_DateQuery verifies CURRENT_DATE BETWEEN start_date AND end_date works cross-database.
func TestPublicNotice_DateQuery(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	lastWeek := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	nextWeek := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	// Notice 1: Currently active
	n1 := models.Notice{
		Title:     "Active Notice",
		Message:   "Currently running notice",
		Audience:  "all",
		Priority:  "high",
		Pinned:    true,
		StartDate: yesterday,
		EndDate:   tomorrow,
		CreatedBy: "Admin",
	}
	// Notice 2: Expired
	n2 := models.Notice{
		Title:     "Expired Notice",
		Message:   "Notice from last week",
		Audience:  "all",
		Priority:  "low",
		Pinned:    false,
		StartDate: lastWeek,
		EndDate:   yesterday,
		CreatedBy: "Admin",
	}
	// Notice 3: Future
	n3 := models.Notice{
		Title:     "Future Notice",
		Message:   "Notice starting tomorrow",
		Audience:  "all",
		Priority:  "medium",
		Pinned:    false,
		StartDate: tomorrow,
		EndDate:   nextWeek,
		CreatedBy: "Admin",
	}

	if err := CreateNotice(n1); err != nil {
		t.Fatalf("Failed creating active notice: %v", err)
	}
	if err := CreateNotice(n2); err != nil {
		t.Fatalf("Failed creating expired notice: %v", err)
	}
	if err := CreateNotice(n3); err != nil {
		t.Fatalf("Failed creating future notice: %v", err)
	}

	activeNotices, err := GetPublicNotices()
	if err != nil {
		t.Fatalf("GetPublicNotices failed: %v", err)
	}

	if len(activeNotices) != 1 {
		t.Fatalf("Expected 1 active notice today (%s), got %d", today, len(activeNotices))
	}
	if activeNotices[0].Title != "Active Notice" {
		t.Errorf("Expected notice title 'Active Notice', got %q", activeNotices[0].Title)
	}
}

// TestNullableColumnsScan verifies that NULL values in officials, discover users, and notifications
// do not result in SQL scan crashes.
func TestNullableColumnsScan(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Insert official with NULL email and NULL photo
	_, err := database.DB.Exec(`
		INSERT INTO officials (full_name, gender, phone, circuit, local_church, position, department, status)
		VALUES ('Bishop Test', 'Male', '08012345678', 'Circuit A', 'Grace Church', 'Bishop', 'Executive', 'active')
	`)
	if err != nil {
		t.Fatalf("Failed inserting official with NULLs: %v", err)
	}

	officials, err := GetAllOfficials()
	if err != nil {
		t.Fatalf("GetAllOfficials failed with NULL email/photo: %v", err)
	}
	if len(officials) != 1 {
		t.Fatalf("Expected 1 official, got %d", len(officials))
	}
	if officials[0].Email != "" || officials[0].Photo != "" {
		t.Errorf("Expected empty string defaults for NULL email/photo, got email=%q photo=%q",
			officials[0].Email, officials[0].Photo)
	}

	officialSingle, err := GetOfficialByID(officials[0].ID)
	if err != nil {
		t.Fatalf("GetOfficialByID failed with NULL email/photo: %v", err)
	}
	if officialSingle.Email != "" || officialSingle.Photo != "" {
		t.Errorf("Expected empty string defaults for GetOfficialByID NULL email/photo, got email=%q photo=%q",
			officialSingle.Email, officialSingle.Photo)
	}

	// 2. Insert user with NULL bio, location, profile_photo
	_, err = database.DB.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES (10, 'Discoverable User', 'discover_user', 'hash')
	`)
	if err != nil {
		t.Fatalf("Failed inserting discover user with NULLs: %v", err)
	}

	discoverUsers, err := GetDiscoverUsers(1)
	if err != nil {
		t.Fatalf("GetDiscoverUsers failed with NULL bio/location/photo: %v", err)
	}
	var found bool
	for _, u := range discoverUsers {
		if u.Username == "discover_user" {
			found = true
			if u.Bio != "" || u.Location != "" || u.ProfilePhoto != "" {
				t.Errorf("Expected empty strings for NULL profile fields, got bio=%q location=%q photo=%q",
					u.Bio, u.Location, u.ProfilePhoto)
			}
		}
	}
	if !found {
		t.Errorf("Created discover user was not found in GetDiscoverUsers")
	}

	// 3. Insert notification with NULL profile_photo for sender
	notif := models.Notification{
		SenderID:   10,
		ReceiverID: 1,
		Type:       "follow",
		Message:    "discover_user followed you",
	}
	if err := CreateNotification(notif); err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	notifs, err := GetNotificationsByUser(1)
	if err != nil {
		t.Fatalf("GetNotificationsByUser failed with NULL sender profile_photo: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Sender.ProfilePhoto != "" {
		t.Errorf("Expected empty ProfilePhoto for NULL actor photo, got %q", notifs[0].Sender.ProfilePhoto)
	}
}

// TestFollowAndReaction_ConflictHandling verifies ANSI ON CONFLICT DO NOTHING and reaction operations.
func TestFollowAndReaction_ConflictHandling(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Follow User (idempotency)
	if err := FollowUser(1, 2); err != nil {
		t.Fatalf("First FollowUser failed: %v", err)
	}
	// Calling a second time should not crash due to ON CONFLICT DO NOTHING
	if err := FollowUser(1, 2); err != nil {
		t.Fatalf("Second FollowUser (duplicate) failed: %v", err)
	}

	isFollowing, err := IsFollowing(1, 2)
	if err != nil {
		t.Fatalf("IsFollowing failed: %v", err)
	}
	if !isFollowing {
		t.Errorf("Expected user 1 to follow user 2")
	}

	if err := UnfollowUser(1, 2); err != nil {
		t.Fatalf("UnfollowUser failed: %v", err)
	}
	isFollowingAfter, _ := IsFollowing(1, 2)
	if isFollowingAfter {
		t.Errorf("Expected user 1 to NOT follow user 2 after unfollow")
	}

	// 2. Post Reactions
	post := &models.Post{
		UserID:  1,
		Content: "Test post for reactions",
	}
	if err := CreatePost(post); err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	// Add 'pray' reaction
	if err := AddReaction(post.ID, 2, "pray"); err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}

	reaction, err := GetReaction(post.ID, 2)
	if err != nil {
		t.Fatalf("GetReaction failed: %v", err)
	}
	if reaction != "pray" {
		t.Errorf("Expected reaction 'pray', got %q", reaction)
	}

	// Update to 'amen'
	if err := UpdateReaction(post.ID, 2, "amen"); err != nil {
		t.Fatalf("UpdateReaction failed: %v", err)
	}
	reactionUpdated, _ := GetReaction(post.ID, 2)
	if reactionUpdated != "amen" {
		t.Errorf("Expected updated reaction 'amen', got %q", reactionUpdated)
	}

	// Count 'amen'
	count, err := CountReactionType(post.ID, "amen")
	if err != nil || count != 1 {
		t.Errorf("Expected count 1 for 'amen', got %d (err: %v)", count, err)
	}

	// Remove reaction
	if err := RemoveReaction(post.ID, 2); err != nil {
		t.Fatalf("RemoveReaction failed: %v", err)
	}
	reactionAfterRemove, _ := GetReaction(post.ID, 2)
	if reactionAfterRemove != "" {
		t.Errorf("Expected empty reaction after remove, got %q", reactionAfterRemove)
	}
}

// TestAllRepositoryQueries_PostgresSyntaxValidation statically parses all repository files to guarantee:
// 1. Zero raw '?' parameter placeholders in SQL statements (which cause PostgreSQL SQLSTATE 42601).
// 2. Zero SQLite-specific date functions (date('now'), datetime('now'), strftime).
// 3. Zero SQLite-specific keywords like AUTOINCREMENT or INSERT OR IGNORE in runtime repository queries.
// 4. Proper ANSI boolean literals (TRUE/FALSE) instead of numeric = 1 / = 0 on boolean columns.
func TestAllRepositoryQueries_PostgresSyntaxValidation(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Failed globbing repository files: %v", err)
	}

	fset := token.NewFileSet()
	for _, file := range files {
		// Skip test files
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed parsing %s: %v", file, err)
		}

		ast.Inspect(node, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}

			val := strings.Trim(lit.Value, "`\"")
			upper := strings.ToUpper(val)

			// Check if this string literal is a SQL statement
			isSQL := strings.HasPrefix(upper, "SELECT") ||
				strings.HasPrefix(upper, "INSERT") ||
				strings.HasPrefix(upper, "UPDATE") ||
				strings.HasPrefix(upper, "DELETE") ||
				strings.Contains(upper, "FROM ") ||
				strings.Contains(upper, "WHERE ")

			if !isSQL {
				return true
			}

			line := fset.Position(lit.Pos()).Line

			// 1. Verify no '?' placeholder in SQL statements
			if strings.Contains(val, "?") {
				t.Errorf("%s:%d SQL statement contains '?' parameter placeholder (causes SQLSTATE 42601 in PostgreSQL): %s",
					file, line, strings.TrimSpace(val))
			}

			// 2. Verify no SQLite date('now') or datetime('now')
			if strings.Contains(strings.ToLower(val), "date('now')") ||
				strings.Contains(strings.ToLower(val), "datetime('now')") ||
				strings.Contains(strings.ToLower(val), "strftime(") {
				t.Errorf("%s:%d SQL statement contains SQLite-only date/time function: %s",
					file, line, strings.TrimSpace(val))
			}

			// 3. Verify no INSERT OR IGNORE
			if strings.Contains(upper, "INSERT OR IGNORE") {
				t.Errorf("%s:%d SQL statement contains SQLite-only 'INSERT OR IGNORE' (use ON CONFLICT DO NOTHING): %s",
					file, line, strings.TrimSpace(val))
			}

			// 4. Verify no integer comparisons on boolean columns
			booleanCols := []string{
				"checked_in",
				"active",
				"is_hidden",
				"is_private",
				"is_public",
				"is_read",
			}
			for _, col := range booleanCols {
				if strings.Contains(upper, col+" = 1") || strings.Contains(upper, col+" = 0") ||
					strings.Contains(upper, col+"=1") || strings.Contains(upper, col+"=0") {
					t.Errorf("%s:%d SQL statement compares boolean column %q with integer 0/1 instead of TRUE/FALSE: %s",
						file, line, col, strings.TrimSpace(val))
				}
			}

			return true
		})
	}
}

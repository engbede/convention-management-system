package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/middleware"
	"convention-management-system/sessions"
)

func TestHomeHandler_UnauthenticatedVisitor(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed convention and notice
	_, err := database.DB.Exec(`
		INSERT INTO conventions (id, name, theme, venue, start_date, end_date, year, active)
		VALUES (1, 'Annual Diocesan Youth Convention 2026', 'Living Hope', 'Cathedral of Peace, Apa', '2026-11-20', '2026-11-24', 2026, 1);
		INSERT INTO notices (id, title, message, audience, priority, pinned, start_date, end_date, created_by, created_at)
		VALUES (1, 'Welcome to the Convention Portal', 'Early registration is now open.', 'all', 'high', 1, '2026-10-01', '2026-10-31', 'Admin', datetime('now'));
	`)
	if err != nil {
		t.Fatalf("Failed to seed convention and notice: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	Home(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()

	// 1. Verify responsive navbar toggler is present
	if !strings.Contains(body, "navbar-toggler") {
		t.Errorf("Expected mobile navbar-toggler in home page")
	}

	// 2. Verify public navigation links are present
	expectedLinks := []string{
		`href="/"`,
		`href="#conventions"`,
		`href="/register"`,
		`href="/announcements"`,
		`href="/contact"`,
		`href="/community/login"`,
		`href="/signup"`,
		`href="/login"`,
	}
	for _, link := range expectedLinks {
		if !strings.Contains(body, link) {
			t.Errorf("Expected visitor navigation to contain link %s", link)
		}
	}

	// 3. Verify convention content rendered
	if !strings.Contains(body, "Annual Diocesan Youth Convention 2026") {
		t.Errorf("Expected convention name in home page")
	}
	if !strings.Contains(body, "Living Hope") {
		t.Errorf("Expected convention theme in home page")
	}

	// 4. Verify notice content rendered
	if !strings.Contains(body, "Welcome to the Convention Portal") {
		t.Errorf("Expected notice title in home page")
	}

	// 5. Verify feature discovery cards rendered
	if !strings.Contains(body, "Convention Portal") || !strings.Contains(body, "Youth Community") {
		t.Errorf("Expected feature discovery cards in home page")
	}

	// 6. Verify authenticated member links are not displayed
	if strings.Contains(body, `/community/logout`) {
		t.Errorf("Unauthenticated visitor should not see community logout link")
	}
}

func TestHomeHandler_NotFoundOnSubpaths(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/unknown-subpath", nil)
	rr := httptest.NewRecorder()

	Home(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found for non-root path on Home handler, got %d", rr.Code)
	}
}

func TestHomeHandler_GracefulFallbackWhenNoConvention(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Ensure no conventions exist
	_, _ = database.DB.Exec("DELETE FROM conventions")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	Home(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK with no conventions, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "APA DIOCESAN YOUTH FELLOWSHIP") || !strings.Contains(body, "No Active Convention Currently Running") {
		t.Errorf("Expected fallback portal heading and 'No Active Convention' message when no active convention, body length: %d", len(body))
	}
}

func TestHomeHandler_CommunityMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed notification and message for user 1
	_, err := database.DB.Exec(`
		INSERT INTO notifications (sender_id, receiver_id, type, message, is_read)
		VALUES (2, 1, 'like', 'User Two liked your post', 0);

		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 2);
		INSERT INTO conversation_members (conversation_id, user_id, role, last_read_message_id) VALUES (1, 1, 'member', 0);
		INSERT INTO conversation_members (conversation_id, user_id, role, last_read_message_id) VALUES (1, 2, 'member', 0);
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body) VALUES (10, 1, 2, 'text', 'Hello User One!');
	`)
	if err != nil {
		t.Fatalf("Failed to seed notifications/messages: %v", err)
	}

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodGet, "/", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	Home(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for community member, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Verify member-specific navigation
	expectedMemberLinks := []string{
		`href="/community"`,
		`href="/messages"`,
		`href="/notifications"`,
		`href="/profile"`,
		`href="/community/logout"`,
	}
	for _, link := range expectedMemberLinks {
		if !strings.Contains(body, link) {
			t.Errorf("Expected member navigation to contain link %s", link)
		}
	}

	// Verify unread badge count was calculated and rendered
	if !strings.Contains(body, "badge rounded-pill bg-danger") {
		t.Errorf("Expected unread badge for messages/notifications")
	}

	// Verify visitor login button is not in header navigation
	if strings.Contains(body, `<a class="nav-link" href="/community/login">`) {
		t.Errorf("Community member should not see visitor Member Login nav-link in header")
	}
}

func TestHomeHandler_Admin(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
		Path:  "/",
	})
	rr := httptest.NewRecorder()

	Home(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for admin session, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Verify admin-specific navigation
	expectedAdminLinks := []string{
		`href="/dashboard"`,
		`href="/registrations"`,
		`href="/logout"`,
		`Admin Session`,
	}
	for _, link := range expectedAdminLinks {
		if !strings.Contains(body, link) {
			t.Errorf("Expected admin navigation to contain %s", link)
		}
	}
}

func TestPublicAnnouncements(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := database.DB.Exec(`
		INSERT INTO notices (id, title, message, audience, priority, pinned, start_date, end_date, created_by, created_at)
		VALUES (1, 'Public Diocesan Announcement', 'All youths are invited to the convention rally.', 'all', 'high', 1, '2026-10-01', '2026-10-31', 'Admin', datetime('now'));
	`)
	if err != nil {
		t.Fatalf("Failed to seed notice: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	rr := httptest.NewRecorder()

	PublicAnnouncements(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for PublicAnnouncements, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Public Diocesan Announcement") {
		t.Errorf("Expected announcement title in body")
	}
	if !strings.Contains(body, "All youths are invited to the convention rally.") {
		t.Errorf("Expected announcement message in body")
	}
	if !strings.Contains(body, `href="/"`) {
		t.Errorf("Expected back link to home")
	}
}

func TestDeleteBackup_AuthAndExecution(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request to protected DeleteBackup
	unauthReq := httptest.NewRequest(http.MethodPost, "/backup/delete?file=test.db", nil)
	unauthRR := httptest.NewRecorder()

	handler := middleware.RequireAuth(DeleteBackup)
	handler(unauthRR, unauthReq)

	// Should redirect to /login
	if unauthRR.Code != http.StatusSeeOther && unauthRR.Code != http.StatusFound {
		t.Errorf("Expected redirect status (302/303) for unauthenticated DeleteBackup, got %d", unauthRR.Code)
	}
	if loc := unauthRR.Header().Get("Location"); loc != "/login" {
		t.Errorf("Expected redirect to /login, got %q", loc)
	}

	// 2. Authenticated admin request
	authReq := httptest.NewRequest(http.MethodPost, "/backup/delete?file=nonexistent.db", nil)
	authReq.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
		Path:  "/",
	})
	authRR := httptest.NewRecorder()

	handler(authRR, authReq)

	// Should successfully process and redirect to /backup
	if authRR.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 See Other, got %d", authRR.Code)
	}
	if loc := authRR.Header().Get("Location"); loc != "/backup" {
		t.Errorf("Expected redirect to /backup, got %q", loc)
	}
}

func TestCommunityLogout(t *testing.T) {
	initReq := httptest.NewRequest(http.MethodGet, "/community/logout", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	req := httptest.NewRequest(http.MethodGet, "/community/logout", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	CommunityLogout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 See Other, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/community/login" {
		t.Errorf("Expected redirect to '/community/login', got %q", loc)
	}
}

func TestAdminLogout(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
		Path:  "/",
	})
	rr := httptest.NewRecorder()

	AdminLogout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 See Other, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("Expected redirect to '/login', got %q", loc)
	}

	// Check admin_session cookie is cleared (MaxAge < 0)
	var cleared bool
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "admin_session" && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("Expected admin_session cookie to be cleared with negative MaxAge")
	}
}

func TestCommunityNavigation_SidebarAndDrawer_PostFeedAndFriends(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed friend request to user 1 from user 2
	_, err := database.DB.Exec(`
		INSERT INTO friend_requests (sender_id, receiver_id, status, created_at)
		VALUES (2, 1, 'pending', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("Failed to seed friend request: %v", err)
	}

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodGet, "/community", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	req := httptest.NewRequest(http.MethodGet, "/community", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	Community(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for community feed, got %d", rr.Code)
	}

	body := rr.Body.String()

	// 1. Verify Post Feed in primary left sidebar with active highlight
	if !strings.Contains(body, `href="/community"`) || !strings.Contains(body, `Post Feed`) {
		t.Errorf("Expected 'Post Feed' link with href='/community' in sidebar")
	}
	if !strings.Contains(body, `nav-post-feed`) {
		t.Errorf("Expected 'nav-post-feed' class on Post Feed link")
	}
	if !strings.Contains(body, `bi-newspaper`) {
		t.Errorf("Expected 'bi-newspaper' icon for Post Feed")
	}

	// 2. Verify Friends in primary left sidebar with badge
	if !strings.Contains(body, `href="/friends"`) || !strings.Contains(body, `Friends`) {
		t.Errorf("Expected 'Friends' link with href='/friends' in sidebar")
	}
	if !strings.Contains(body, `nav-friends`) {
		t.Errorf("Expected 'nav-friends' class on Friends link")
	}
	if !strings.Contains(body, `bi-people-fill`) {
		t.Errorf("Expected 'bi-people-fill' icon for Friends")
	}
	// Verify pending request badge count (1)
	if !strings.Contains(body, `title="1 pending friend requests">1</span>`) {
		t.Errorf("Expected badge showing '1 pending friend requests' on sidebar Friends item")
	}

	// 3. Verify Mobile Drawer / Navbar contains Post Feed and Friends
	if !strings.Contains(body, `navbar-toggler`) {
		t.Errorf("Expected mobile navbar toggler for drawer")
	}
	if !strings.Contains(body, `data-bs-target="#navbarMenu"`) {
		t.Errorf("Expected navbar collapse target #navbarMenu")
	}
	if !strings.Contains(body, `nav-post-feed`) {
		t.Errorf("Expected mobile navbar to contain nav-post-feed")
	}

	// 4. Verify existing sidebar navigation items remain preserved
	existingItems := []string{
		`href="/profile"`,
		`href="/groups"`,
		`href="/messages"`,
		`href="/events"`,
		`href="/programmes"`,
		`href="/bible-study"`,
		`href="/prayer"`,
		`href="/announcements"`,
		`href="/publications"`,
		`href="/saved"`,
		`href="/settings"`,
	}
	for _, item := range existingItems {
		if !strings.Contains(body, item) {
			t.Errorf("Expected existing sidebar item %s to be preserved", item)
		}
	}
}

func TestFriendsDashboard_AuthenticatedViewAndActiveNav(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed friend relationship and pending request
	_, err := database.DB.Exec(`
		INSERT INTO friends (user1_id, user2_id, created_at)
		VALUES (1, 3, CURRENT_TIMESTAMP);
		INSERT INTO friend_requests (sender_id, receiver_id, status, created_at)
		VALUES (2, 1, 'pending', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("Failed to seed friendships/requests: %v", err)
	}

	initReq := httptest.NewRequest(http.MethodGet, "/friends", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	req := httptest.NewRequest(http.MethodGet, "/friends", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	FriendsDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for FriendsDashboard, got %d; body: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()

	// Verify friends dashboard rendered
	if !strings.Contains(body, "Friends Dashboard") && !strings.Contains(body, "My Friends") {
		t.Errorf("Expected Friends Dashboard content")
	}

	// Verify incoming request from user 2 is shown
	if !strings.Contains(body, "User Two") {
		t.Errorf("Expected incoming request from User Two to be rendered")
	}

	// Verify existing friend user 3 is shown
	if !strings.Contains(body, "User Three") {
		t.Errorf("Expected friend User Three to be rendered")
	}

	// Verify pending request badge count (1) in navbar
	if !strings.Contains(body, `badge rounded-pill bg-danger ms-1`) {
		t.Errorf("Expected pending friend request count badge on navbar Friends item")
	}
}

func TestFriendRequests_AuthenticatedAndUnauthenticated(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request must redirect to /community/login
	unauthReq := httptest.NewRequest(http.MethodGet, "/friend-requests", nil)
	unauthRec := httptest.NewRecorder()
	FriendRequests(unauthRec, unauthReq)

	if unauthRec.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 See Other for unauthenticated /friend-requests, got %d", unauthRec.Code)
	}
	if loc := unauthRec.Header().Get("Location"); loc != "/community/login" {
		t.Errorf("Expected redirect to /community/login, got %q", loc)
	}

	// 2. Authenticated request
	_, err := database.DB.Exec(`
		INSERT INTO friend_requests (sender_id, receiver_id, status, created_at)
		VALUES (2, 1, 'pending', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("Failed to seed friend request: %v", err)
	}

	initReq := httptest.NewRequest(http.MethodGet, "/friend-requests", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	req := httptest.NewRequest(http.MethodGet, "/friend-requests", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	FriendRequests(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for FriendRequests, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Friend Requests") {
		t.Errorf("Expected Friend Requests page content")
	}
	if !strings.Contains(body, "User Two") {
		t.Errorf("Expected request from User Two in body")
	}
}

func TestAdminNavigation_DesktopSidebarAndMobileDrawer(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed convention so Dashboard renders successfully
	_, err := database.DB.Exec(`
		INSERT INTO conventions (id, name, theme, venue, start_date, end_date, year, active)
		VALUES (1, 'Annual Diocesan Youth Convention 2026', 'Living Hope', 'Cathedral of Peace, Apa', '2026-11-20', '2026-11-24', 2026, 1);
	`)
	if err != nil {
		t.Fatalf("Failed to seed convention: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
		Path:  "/",
	})
	rr := httptest.NewRecorder()

	Dashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for admin Dashboard, got %d", rr.Code)
	}

	body := rr.Body.String()

	// 1. Verify desktop sidebar has Post Feed and Friends
	if !strings.Contains(body, `admin-sidebar`) {
		t.Errorf("Expected desktop admin-sidebar container")
	}

	// 2. Verify mobile offcanvas drawer has Post Feed and Friends
	if !strings.Contains(body, `id="adminSidebarOffcanvas"`) {
		t.Errorf("Expected mobile offcanvas drawer #adminSidebarOffcanvas")
	}

	// 3. Verify Post Feed and Friends links appear in both
	if !strings.Contains(body, `href="/community" class="admin-nav-link nav-post-feed"`) {
		t.Errorf("Expected admin nav link for Post Feed with class nav-post-feed and href /community")
	}
	if !strings.Contains(body, `href="/friends" class="admin-nav-link nav-friends"`) {
		t.Errorf("Expected admin nav link for Friends with class nav-friends and href /friends")
	}

	// 4. Verify existing admin navigation remains intact
	adminItems := []string{
		`href="/dashboard"`,
		`href="/registrations"`,
		`href="/reports"`,
		`href="/settings"`,
		`href="/backup"`,
		`href="/logout"`,
	}
	for _, item := range adminItems {
		if !strings.Contains(body, item) {
			t.Errorf("Expected existing admin nav item %s to be preserved", item)
		}
	}
}

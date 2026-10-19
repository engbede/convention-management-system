package services

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"

	"convention-management-system/database"
	"convention-management-system/repository"

	_ "github.com/mattn/go-sqlite3"
)

func setupGroupCallTestDB(t *testing.T) (func(), int, []int) {
	t.Helper()
	testDBPath := "test_group_call.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Insert 20 users
	for i := 1; i <= 20; i++ {
		_, err = db.Exec(`
			INSERT INTO users (id, full_name, username, password_hash)
			VALUES (?, ?, ?, 'hash');
		`, i, fmt.Sprintf("User %d", i), fmt.Sprintf("user%d", i))
		if err != nil {
			t.Fatalf("Failed to insert user %d: %v", i, err)
		}
	}

	// Create a community group conversation with creator = User 1
	var initialMemberIDs []int
	for i := 2; i <= 18; i++ {
		initialMemberIDs = append(initialMemberIDs, i)
	}

	conv, err := CreateGroup(1, "Youth Prayer Group", "prayer", "A group for prayer and fellowship", "all", true, initialMemberIDs)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Set roles: User 2: admin, User 3: moderator
	_ = repository.UpdateConversationMemberRole(conv.ID, 2, "admin")
	_ = repository.UpdateConversationMemberRole(conv.ID, 3, "moderator")
	// Users 19, 20 are outsiders (not members)

	cleanup := func() {
		// Clean up in-memory rooms
		GlobalGroupCallManager.mu.Lock()
		delete(GlobalGroupCallManager.rooms, conv.ID)
		GlobalGroupCallManager.mu.Unlock()

		db.Close()
		_ = os.Remove(testDBPath)
	}

	return cleanup, conv.ID, initialMemberIDs
}

func TestGroupCallLifecycle(t *testing.T) {
	cleanup, convID, _ := setupGroupCallTestDB(t)
	defer cleanup()

	// 1. Outsider (User 19) cannot start call
	_, err := GlobalGroupCallManager.StartGroupCall(convID, 19, "audio")
	if err == nil {
		t.Errorf("Expected error when outsider attempts to start group call, got nil")
	}

	// 2. Member (User 1 - owner) starts video call
	room, err := GlobalGroupCallManager.StartGroupCall(convID, 1, "video")
	if err != nil {
		t.Fatalf("Failed to start group call: %v", err)
	}
	if room == nil {
		t.Fatalf("Expected room pointer, got nil")
	}
	if room.CallID <= 0 {
		t.Errorf("Expected positive CallID, got %d", room.CallID)
	}
	if room.CallType != "video" {
		t.Errorf("Expected call_type 'video', got %s", room.CallType)
	}
	if len(room.Participants) != 1 {
		t.Errorf("Expected 1 participant initially, got %d", len(room.Participants))
	}
	p1, ok := room.Participants[1]
	if !ok || p1.Role != "owner" {
		t.Errorf("Expected initiator User 1 to have role 'owner', got %+v", p1)
	}

	// Verify call log persisted in DB
	logs, err := repository.GetConversationCallLogs(convID, 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("Expected call log in DB, got error: %v, count: %d", err, len(logs))
	}
	if logs[0].CallerID != 1 {
		t.Errorf("Expected caller_id 1, got %d", logs[0].CallerID)
	}

	// 3. User 2 (admin) joins call
	room, p2, err := GlobalGroupCallManager.JoinGroupCall(convID, 2)
	if err != nil {
		t.Fatalf("User 2 failed to join call: %v", err)
	}
	if p2 == nil || p2.Role != "admin" {
		t.Errorf("Expected User 2 with role 'admin', got %+v", p2)
	}
	if len(room.Participants) != 2 {
		t.Errorf("Expected 2 participants in room, got %d", len(room.Participants))
	}

	// Verify call participants in DB
	participants, err := repository.GetCallParticipants(room.CallID)
	if err != nil || len(participants) != 2 {
		t.Errorf("Expected 2 participants in DB, got error: %v, count: %d", err, len(participants))
	}

	// 4. Outsider (User 20) attempts to join call -> should fail
	_, _, err = GlobalGroupCallManager.JoinGroupCall(convID, 20)
	if err == nil {
		t.Errorf("Expected error when outsider joins call, got nil")
	}

	// 5. Update media state (User 2 mutes and turns cam off)
	updatedP2, err := GlobalGroupCallManager.UpdateParticipantMediaState(convID, 2, true, true, false)
	if err != nil {
		t.Fatalf("Failed to update media state: %v", err)
	}
	if !updatedP2.IsMuted || !updatedP2.IsCamOff || updatedP2.IsSpeaking {
		t.Errorf("Media state mismatch: IsMuted=%v, IsCamOff=%v, IsSpeaking=%v", updatedP2.IsMuted, updatedP2.IsCamOff, updatedP2.IsSpeaking)
	}

	// Speaking state update for User 1
	updatedP1, err := GlobalGroupCallManager.UpdateParticipantMediaState(convID, 1, false, false, true)
	if err != nil {
		t.Fatalf("Failed to update speaking state: %v", err)
	}
	if !updatedP1.IsSpeaking {
		t.Errorf("Expected User 1 to be speaking, got false")
	}

	// 6. User 4 (regular member) joins call
	_, _, err = GlobalGroupCallManager.JoinGroupCall(convID, 4)
	if err != nil {
		t.Fatalf("User 4 failed to join: %v", err)
	}

	// User 4 (member) tries to kick User 2 (admin) -> should fail
	err = GlobalGroupCallManager.KickParticipantFromGroupCall(convID, 4, 2)
	if err == nil {
		t.Errorf("Expected error when regular member tries to kick admin, got nil")
	}

	// User 2 (admin) tries to kick User 1 (owner) -> should fail (cannot remove higher rank)
	err = GlobalGroupCallManager.KickParticipantFromGroupCall(convID, 2, 1)
	if err == nil {
		t.Errorf("Expected error when admin tries to kick owner, got nil")
	}

	// User 2 (admin) kicks User 4 (regular member) -> should succeed
	err = GlobalGroupCallManager.KickParticipantFromGroupCall(convID, 2, 4)
	if err != nil {
		t.Errorf("Expected admin to kick regular member successfully, got %v", err)
	}
	if _, stillIn := room.Participants[4]; stillIn {
		t.Errorf("Expected User 4 to be removed from room, but still present")
	}

	// 7. User 2 leaves call
	room, duration, err := GlobalGroupCallManager.LeaveGroupCall(convID, 2)
	if err != nil {
		t.Fatalf("User 2 failed to leave call: %v", err)
	}
	if duration <= 0 {
		t.Errorf("Expected positive duration, got %d", duration)
	}
	if len(room.Participants) != 1 {
		t.Errorf("Expected 1 participant remaining, got %d", len(room.Participants))
	}

	// Verify room is still active
	activeRoom, exists := GlobalGroupCallManager.GetActiveGroupCall(convID)
	if !exists || activeRoom == nil {
		t.Errorf("Expected group call to still be active with User 1 remaining")
	}

	// 8. User 3 (regular moderator, not initiator or owner/admin) tries to end call for everyone
	_, err = GlobalGroupCallManager.EndGroupCall(convID, 3)
	if err == nil {
		t.Errorf("Expected error when moderator/non-admin ends call for everyone, got nil")
	}

	// 9. User 1 (initiator and owner) ends call for everyone
	endedRoom, err := GlobalGroupCallManager.EndGroupCall(convID, 1)
	if err != nil {
		t.Fatalf("Failed to end group call: %v", err)
	}
	if endedRoom == nil {
		t.Fatalf("Expected ended room, got nil")
	}

	// Verify room no longer active
	_, exists = GlobalGroupCallManager.GetActiveGroupCall(convID)
	if exists {
		t.Errorf("Expected group call to no longer be active in manager")
	}

	// Verify call log completed in DB
	logs, err = repository.GetConversationCallLogs(convID, 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("Failed to fetch logs: %v", err)
	}
	if logs[0].Status != "completed" {
		t.Errorf("Expected call log status 'completed', got %s", logs[0].Status)
	}
}

func TestGroupCallAutoTerminationOnLastLeave(t *testing.T) {
	cleanup, convID, _ := setupGroupCallTestDB(t)
	defer cleanup()

	// Start call with User 1
	room, err := GlobalGroupCallManager.StartGroupCall(convID, 1, "audio")
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// User 1 leaves
	_, _, err = GlobalGroupCallManager.LeaveGroupCall(convID, 1)
	if err != nil {
		t.Fatalf("Failed to leave call: %v", err)
	}

	// Room should automatically terminate and be deleted
	_, exists := GlobalGroupCallManager.GetActiveGroupCall(convID)
	if exists {
		t.Errorf("Expected room to automatically terminate when last participant leaves")
	}

	// DB call log should be completed
	logs, err := repository.GetConversationCallLogs(convID, 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("Failed to fetch logs: %v", err)
	}
	if logs[0].Status != "completed" {
		t.Errorf("Expected call log to be 'completed', got %s", logs[0].Status)
	}
	_ = room
}

func TestGroupCallCapacityLimit(t *testing.T) {
	cleanup, convID, memberIDs := setupGroupCallTestDB(t)
	defer cleanup()

	// Verify community room default limit is extended above 300
	if defaultLimit := GlobalGroupCallManager.GetMaxParticipants(); defaultLimit < 300 {
		t.Fatalf("Expected default group call capacity to be at least 300, got %d", defaultLimit)
	}

	// For capacity limit testing, temporarily set limit to 16
	GlobalGroupCallManager.SetMaxParticipants(16)
	defer GlobalGroupCallManager.SetMaxParticipants(DefaultMaxGroupCallParticipants)

	// Start call with User 1 (1st participant)
	room, err := GlobalGroupCallManager.StartGroupCall(convID, 1, "video")
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// User 2 and User 3 join (2nd and 3rd participants)
	_, _, _ = GlobalGroupCallManager.JoinGroupCall(convID, 2)
	_, _, _ = GlobalGroupCallManager.JoinGroupCall(convID, 3)

	// Users 4 to 16 join (total 16 participants reached: 1 to 16)
	for i := 4; i <= 16; i++ {
		_, _, err := GlobalGroupCallManager.JoinGroupCall(convID, i)
		if err != nil {
			t.Fatalf("User %d failed to join: %v", i, err)
		}
	}

	if len(room.Participants) != 16 {
		t.Fatalf("Expected 16 participants in room, got %d", len(room.Participants))
	}

	// 17th user (User 17) attempts to join -> must be rejected due to capacity limit
	_, _, err = GlobalGroupCallManager.JoinGroupCall(convID, 17)
	if err == nil {
		t.Errorf("Expected capacity limit error for 17th participant, got nil")
	}

	// User 16 leaves the call
	_, _, err = GlobalGroupCallManager.LeaveGroupCall(convID, 16)
	if err != nil {
		t.Fatalf("User 16 failed to leave: %v", err)
	}

	// Now room has 15 participants, User 17 should now be able to join!
	_, p17, err := GlobalGroupCallManager.JoinGroupCall(convID, 17)
	if err != nil {
		t.Fatalf("Expected User 17 to successfully join after capacity freed, got error: %v", err)
	}
	if p17 == nil || p17.UserID != 17 {
		t.Errorf("Expected User 17 participant info, got %+v", p17)
	}
	_ = memberIDs
}

func TestGroupCallPermissionsAdminsOnly(t *testing.T) {
	cleanup, convID, _ := setupGroupCallTestDB(t)
	defer cleanup()

	// Update call permissions to 'admin_only'
	_, err := database.DB.Exec(`UPDATE conversations SET call_permission = 'admin_only' WHERE id = ?`, convID)
	if err != nil {
		t.Fatalf("Failed to update call permissions: %v", err)
	}

	// Regular member (User 4) tries to start call -> must fail
	_, err = GlobalGroupCallManager.StartGroupCall(convID, 4, "audio")
	if err == nil {
		t.Errorf("Expected error when member tries to start call in admin_only group, got nil")
	}

	// Admin (User 2) starts call -> succeeds
	room, err := GlobalGroupCallManager.StartGroupCall(convID, 2, "audio")
	if err != nil {
		t.Fatalf("Admin failed to start call: %v", err)
	}

	// Regular member (User 4) CAN join the call once started
	_, p4, err := GlobalGroupCallManager.JoinGroupCall(convID, 4)
	if err != nil {
		t.Fatalf("Member failed to join existing call: %v", err)
	}
	if p4 == nil {
		t.Errorf("Expected participant 4, got nil")
	}

	// Clean up room
	_, _ = GlobalGroupCallManager.EndGroupCall(convID, 2)
	_ = room
}

func TestGroupCall_300SimultaneousParticipantsAndActiveSpeaker(t *testing.T) {
	testDBPath := "test_group_call_300.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}()
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Insert 300 users in a single transaction for high performance
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin tx: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO users (id, full_name, username, password_hash) VALUES (?, ?, ?, 'hash')`)
	if err != nil {
		t.Fatalf("Failed to prepare user insert: %v", err)
	}
	for i := 1; i <= 300; i++ {
		_, _ = stmt.Exec(i, fmt.Sprintf("Delegate %d", i), fmt.Sprintf("delegate_%d", i))
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit user inserts: %v", err)
	}

	// Create group conversation with user 1 as owner and users 2..300 as members
	var memberIDs []int
	for i := 2; i <= 300; i++ {
		memberIDs = append(memberIDs, i)
	}
	conv, err := CreateGroup(1, "Plenary Convention Hall", "general", "General convention fellowship", "all", true, memberIDs)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	defer func() {
		GlobalGroupCallManager.mu.Lock()
		delete(GlobalGroupCallManager.rooms, conv.ID)
		GlobalGroupCallManager.mu.Unlock()
	}()

	// Verify maximum participants limit is at least 300
	if GlobalGroupCallManager.GetMaxParticipants() < 300 {
		t.Fatalf("Expected room limit >= 300, got %d", GlobalGroupCallManager.GetMaxParticipants())
	}

	// 1. Initiator (User 1) starts call
	room, err := GlobalGroupCallManager.StartGroupCall(conv.ID, 1, "video")
	if err != nil {
		t.Fatalf("Failed to start group call: %v", err)
	}

	// 2. Concurrently join remaining 299 participants (User 2 to User 300)
	var wg sync.WaitGroup
	errChan := make(chan error, 300)

	for i := 2; i <= 300; i++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			_, _, jErr := GlobalGroupCallManager.JoinGroupCall(conv.ID, uid)
			if jErr != nil {
				errChan <- fmt.Errorf("user %d failed to join: %w", uid, jErr)
			}
		}(i)
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Fatalf("Concurrent join error: %v", err)
	}

	// 3. Verify exactly 300 participants are actively enrolled in the room
	room, exists := GlobalGroupCallManager.GetActiveGroupCall(conv.ID)
	if !exists {
		t.Fatalf("Expected active group call to exist")
	}

	GlobalGroupCallManager.mu.RLock()
	totalParticipants := len(room.Participants)
	GlobalGroupCallManager.mu.RUnlock()

	if totalParticipants != 300 {
		t.Fatalf("Expected 300 simultaneous participants, got %d", totalParticipants)
	}

	// 4. Test Active-Speaker Detection & Highlighting
	// Delegate 42 starts speaking
	_, err = GlobalGroupCallManager.UpdateParticipantMediaState(conv.ID, 42, false, false, true)
	if err != nil {
		t.Fatalf("Failed updating active speaker state: %v", err)
	}

	GlobalGroupCallManager.mu.RLock()
	p42 := room.Participants[42]
	GlobalGroupCallManager.mu.RUnlock()

	if p42 == nil || !p42.IsSpeaking {
		t.Errorf("Expected participant 42 to have IsSpeaking=true")
	}

	// Delegate 42 stops speaking, Delegate 105 begins speaking
	_, _ = GlobalGroupCallManager.UpdateParticipantMediaState(conv.ID, 42, false, false, false)
	_, _ = GlobalGroupCallManager.UpdateParticipantMediaState(conv.ID, 105, false, false, true)

	GlobalGroupCallManager.mu.RLock()
	p42After := room.Participants[42]
	p105 := room.Participants[105]
	GlobalGroupCallManager.mu.RUnlock()

	if p42After.IsSpeaking {
		t.Errorf("Expected participant 42 to have IsSpeaking=false after ceasing speech")
	}
	if p105 == nil || !p105.IsSpeaking {
		t.Errorf("Expected participant 105 to be active highlighted speaker")
	}

	// 5. End group call cleanly
	_, err = GlobalGroupCallManager.EndGroupCall(conv.ID, 1)
	if err != nil {
		t.Fatalf("Failed to end group call: %v", err)
	}

	_, stillExists := GlobalGroupCallManager.GetActiveGroupCall(conv.ID)
	if stillExists {
		t.Errorf("Expected group call to be removed after end")
	}
}

func TestGroupCall_RemoteMuteAndModerationPermissions(t *testing.T) {
	cleanup, convID, _ := setupGroupCallTestDB(t)
	defer cleanup()

	// 1. Owner (User 1) starts call
	room, err := GlobalGroupCallManager.StartGroupCall(convID, 1, "video")
	if err != nil {
		t.Fatalf("Failed to start group call: %v", err)
	}

	// 2. Admin (User 2), Moderator (User 3), and Member (User 4) join call
	_, _, err = GlobalGroupCallManager.JoinGroupCall(convID, 2)
	if err != nil {
		t.Fatalf("User 2 failed to join: %v", err)
	}
	_, _, err = GlobalGroupCallManager.JoinGroupCall(convID, 3)
	if err != nil {
		t.Fatalf("User 3 failed to join: %v", err)
	}
	_, _, err = GlobalGroupCallManager.JoinGroupCall(convID, 4)
	if err != nil {
		t.Fatalf("User 4 failed to join: %v", err)
	}

	// Member 4 is unmuted and speaking
	_, err = GlobalGroupCallManager.UpdateParticipantMediaState(convID, 4, false, false, true)
	if err != nil {
		t.Fatalf("Failed to set speaking state for User 4: %v", err)
	}

	// 3. Regular Member (User 4) attempts to remote mute Admin (User 2) -> must fail
	err = GlobalGroupCallManager.MuteParticipantInGroupCall(convID, 4, 2)
	if err == nil {
		t.Errorf("Expected error when regular member attempts remote mute, got nil")
	}

	// 4. Admin (User 2) attempts to remote mute Owner (User 1) -> must fail
	err = GlobalGroupCallManager.MuteParticipantInGroupCall(convID, 2, 1)
	if err == nil {
		t.Errorf("Expected error when admin attempts to remote mute owner, got nil")
	}

	// 5. Admin (User 2) remotely mutes noisy Member (User 4) -> must succeed
	err = GlobalGroupCallManager.MuteParticipantInGroupCall(convID, 2, 4)
	if err != nil {
		t.Fatalf("Admin failed to remote mute member: %v", err)
	}

	GlobalGroupCallManager.mu.RLock()
	p4 := room.Participants[4]
	GlobalGroupCallManager.mu.RUnlock()

	if p4 == nil {
		t.Fatalf("Expected User 4 in participants")
	}
	if !p4.IsMuted {
		t.Errorf("Expected User 4 to have IsMuted=true after remote mute")
	}
	if p4.IsSpeaking {
		t.Errorf("Expected User 4 to have IsSpeaking=false after remote mute")
	}

	// 6. Moderator (User 3) remotely mutes User 4 again or unmutes then mutes
	err = GlobalGroupCallManager.MuteParticipantInGroupCall(convID, 3, 4)
	if err != nil {
		t.Errorf("Moderator should be able to remote mute member: %v", err)
	}

	// 7. Remote mute on user not in call (User 10) -> must fail
	err = GlobalGroupCallManager.MuteParticipantInGroupCall(convID, 1, 10)
	if err == nil {
		t.Errorf("Expected error when muting user not in call, got nil")
	}

	// 8. End call
	_, err = GlobalGroupCallManager.EndGroupCall(convID, 1)
	if err != nil {
		t.Fatalf("Failed to end call: %v", err)
	}
}



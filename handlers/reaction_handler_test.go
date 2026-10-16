package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"convention-management-system/database"
)

func TestReactionHandler(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert a test post
	res, err := database.DB.Exec(`
		INSERT INTO posts (user_id, content, created_at)
		VALUES (1, 'Test community post for reactions', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test post: %v", err)
	}
	postID, _ := res.LastInsertId()

	// 1. React with valid reaction via JSON (user 2 reacts with 'love')
	reactJSON := fmt.Sprintf(`{"post_id": %d, "reaction": "love"}`, postID)
	req1, rr1 := makeAuthReq(http.MethodPost, "/community/react", reactJSON, "application/json", 2)
	ReactToPost(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("ReactToPost failed: %d (body: %s)", rr1.Code, rr1.Body.String())
	}

	var resp1 struct {
		Success bool `json:"success"`
		Stats   struct {
			PostID            int    `json:"post_id"`
			UserReaction      string `json:"user_reaction"`
			UserReactionEmoji string `json:"user_reaction_emoji"`
			TotalReactions    int    `json:"total_reactions"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(rr1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("Failed to parse ReactToPost JSON response: %v", err)
	}
	if !resp1.Success || resp1.Stats.UserReaction != "love" || resp1.Stats.TotalReactions != 1 {
		t.Errorf("Unexpected stats after reaction: %+v", resp1.Stats)
	}

	// 2. React with invalid reaction type -> should return 400 Bad Request
	badJSON := fmt.Sprintf(`{"post_id": %d, "reaction": "invalid_reaction_type"}`, postID)
	badReq, badRR := makeAuthReq(http.MethodPost, "/community/react", badJSON, "application/json", 2)
	ReactToPost(badRR, badReq)
	if badRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid reaction, got %d", badRR.Code)
	}

	// 3. Toggle off reaction by clicking 'love' again
	toggleReq, toggleRR := makeAuthReq(http.MethodPost, "/community/react", reactJSON, "application/json", 2)
	ReactToPost(toggleRR, toggleReq)
	if toggleRR.Code != http.StatusOK {
		t.Fatalf("ReactToPost toggle off failed: %d", toggleRR.Code)
	}
	var toggleResp struct {
		Success bool `json:"success"`
		Stats   struct {
			TotalReactions int    `json:"total_reactions"`
			UserReaction   string `json:"user_reaction"`
		} `json:"stats"`
	}
	_ = json.Unmarshal(toggleRR.Body.Bytes(), &toggleResp)
	if toggleResp.Stats.TotalReactions != 0 || toggleResp.Stats.UserReaction != "" {
		t.Errorf("Expected 0 reactions after toggle off, got %d (user_reaction: %q)", toggleResp.Stats.TotalReactions, toggleResp.Stats.UserReaction)
	}

	// 4. Test GetPostReactionsAPI
	getURL := fmt.Sprintf("/community/post/reactions?post_id=%d", postID)
	getReq, getRR := makeAuthReq(http.MethodGet, getURL, "", "", 2)
	GetPostReactionsAPI(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("GetPostReactionsAPI failed: %d (body: %s)", getRR.Code, getRR.Body.String())
	}
	var getResp struct {
		Success bool `json:"success"`
		Catalog []struct {
			Category string `json:"category"`
			Title    string `json:"title"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("Failed to parse GetPostReactionsAPI JSON: %v", err)
	}
	if len(getResp.Catalog) != 8 {
		t.Errorf("Expected 8 curated reaction categories, got %d", len(getResp.Catalog))
	}
}

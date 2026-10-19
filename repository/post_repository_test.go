package repository_test

import (
	"database/sql"
	"os"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"

	_ "github.com/mattn/go-sqlite3"
)

func setupPostTestDB(t *testing.T) func() {
	t.Helper()
	testDBPath := "test_posts.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to migrate SQLite: %v", err)
	}

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}
}

func TestPostImageURL(t *testing.T) {
	cases := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "empty image",
			image:    "",
			expected: "",
		},
		{
			name:     "standard relative upload",
			image:    "posts/sample123.jpg",
			expected: "/uploads/posts/sample123.jpg",
		},
		{
			name:     "already prefixed uploads",
			image:    "/uploads/posts/sample123.jpg",
			expected: "/uploads/posts/sample123.jpg",
		},
		{
			name:     "external Cloudinary URL",
			image:    "https://res.cloudinary.com/diocesan/image/upload/v1234/test.png",
			expected: "https://res.cloudinary.com/diocesan/image/upload/v1234/test.png",
		},
		{
			name:     "external HTTP URL",
			image:    "http://example.com/photo.jpg",
			expected: "http://example.com/photo.jpg",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := models.Post{Image: tc.image}
			if got := p.ImageURL(); got != tc.expected {
				t.Errorf("ImageURL() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestGetAllPosts_And_GetPostsByUser_WithNullableColumns(t *testing.T) {
	cleanup := setupPostTestDB(t)
	defer cleanup()

	// 1. Insert test user with NULL profile_photo
	res, err := database.DB.Exec(`
		INSERT INTO users (full_name, username, email, password_hash, role, profile_photo)
		VALUES ('Barnabas Idoko', 'barnabas', 'barnabas@example.com', 'hash', 'member', NULL)
	`)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	userID, _ := res.LastInsertId()

	// 2. Insert post with NULL image, video, likes, comments, shares
	_, err = database.DB.Exec(`
		INSERT INTO posts (user_id, content, image, video, visibility, likes, comments, shares, is_edited)
		VALUES (?, 'Youth conference announcement', NULL, NULL, NULL, NULL, NULL, NULL, NULL)
	`, userID)
	if err != nil {
		t.Fatalf("Failed to insert post with nulls: %v", err)
	}

	// 3. Insert post with actual image
	_, err = database.DB.Exec(`
		INSERT INTO posts (user_id, content, image, video, visibility, likes, comments, shares, is_edited)
		VALUES (?, 'Youth Fellowship banner photo', 'posts/banner.jpg', '', 'public', 5, 2, 1, 0)
	`, userID)
	if err != nil {
		t.Fatalf("Failed to insert post with image: %v", err)
	}

	// 4. GetAllPosts should succeed without scan errors on NULL columns
	posts, err := repository.GetAllPosts()
	if err != nil {
		t.Fatalf("GetAllPosts() failed with error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("Expected 2 posts, got %d", len(posts))
	}

	// Verify image resolution on posts
	var foundWithImage, foundWithoutImage bool
	for _, p := range posts {
		if p.Image == "posts/banner.jpg" {
			foundWithImage = true
			if p.ImageURL() != "/uploads/posts/banner.jpg" {
				t.Errorf("Expected ImageURL /uploads/posts/banner.jpg, got %s", p.ImageURL())
			}
		} else if p.Image == "" {
			foundWithoutImage = true
			if p.ImageURL() != "" {
				t.Errorf("Expected empty ImageURL, got %s", p.ImageURL())
			}
		}
	}
	if !foundWithImage || !foundWithoutImage {
		t.Errorf("Did not find expected posts with/without image")
	}

	// 5. GetPostsByUser should also succeed without scan errors
	userPosts, err := repository.GetPostsByUser(int(userID))
	if err != nil {
		t.Fatalf("GetPostsByUser() failed with error: %v", err)
	}
	if len(userPosts) != 2 {
		t.Fatalf("Expected 2 user posts, got %d", len(userPosts))
	}
}


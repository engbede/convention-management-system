package repository

import (
	"strings"
	"testing"
	"time"

	"convention-management-system/models"
)

func TestCreateUser_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := &models.User{
		FullName:              "Test Youth Member",
		Username:              "youth_member_1",
		Email:                 "member1@example.com",
		Phone:                 "08012345678",
		PasswordHash:          "$2a$10$hashedpasswordplaceholder123",
		Role:                  "attendee",
		Bio:                   "Youth fellowship member and singer",
		Gender:                "Female",
		BirthDate:             "2002-05-15",
		Occupation:            "Software Developer",
		Country:               "Nigeria",
		State:                 "Benue",
		Location:              "Makurdi",
		Website:               "https://example.com",
		ProfilePhoto:          "/uploads/profile.png",
		CoverPhoto:            "/uploads/cover.png",
		ChurchName:            "Wesley Cathedral",
		Circuit:               "Makurdi Circuit",
		LocalChurch:           "Grace Society",
		Department:            "Choir",
		Position:              "Secretary",
		FavoriteBibleVerse:    "Jeremiah 29:11",
		LifeVerse:             "Philippians 4:13",
		Calling:               "Music Ministry",
		SpiritualGifts:        "Singing, Encouragement",
		SalvationTestimony:    "Saved in 2018 at youth camp",
		FavoritePreacher:      "John Wesley",
		FavoriteChristianBook: "Mere Christianity",
		FavoriteWorshipSong:   "10,000 Reasons",
		FavoriteGospelArtist:  "Sinach",
		Skills:                "Go, TypeScript, Singing",
		Languages:             "English, Tiv",
		Hobbies:               "Reading, Traveling",
		Mission:               "To reach youth for Christ",
		Vision:                "A vibrant community of believers",
		FavoriteQuote:         "Faith over fear",
		WaterBaptized:         true,
		HolySpiritBaptized:    true,
		IsVerified:            false,
		IsActive:              true,
		Followers:             5,
		Following:             10,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed with error: %v", err)
	}

	if user.ID == 0 {
		t.Errorf("Expected user.ID to be populated after CreateUser, got 0")
	}

	// Retrieve by username
	fetchedUser, err := GetUserByUsername(user.Username)
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}

	if fetchedUser.FullName != user.FullName {
		t.Errorf("Expected FullName %q, got %q", user.FullName, fetchedUser.FullName)
	}
	if fetchedUser.Email != user.Email {
		t.Errorf("Expected Email %q, got %q", user.Email, fetchedUser.Email)
	}
	if fetchedUser.Phone != user.Phone {
		t.Errorf("Expected Phone %q, got %q", user.Phone, fetchedUser.Phone)
	}
	if fetchedUser.PasswordHash != user.PasswordHash {
		t.Errorf("Expected PasswordHash to match")
	}
	if !fetchedUser.IsActive {
		t.Errorf("Expected IsActive to be true")
	}

	// Retrieve by ID
	byID, err := GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if byID.Username != user.Username {
		t.Errorf("Expected Username %q, got %q", user.Username, byID.Username)
	}
	if byID.ChurchName != user.ChurchName {
		t.Errorf("Expected ChurchName %q, got %q", user.ChurchName, byID.ChurchName)
	}
}

func TestCreateUser_DefaultFields(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	minimalUser := &models.User{
		FullName:     "Minimal User",
		Username:     "minimal_user",
		PasswordHash: "secret_hash",
	}

	err := CreateUser(minimalUser)
	if err != nil {
		t.Fatalf("CreateUser with minimal fields failed: %v", err)
	}

	fetched, err := GetUserByUsername("minimal_user")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}

	if fetched.Role != "attendee" {
		t.Errorf("Expected default role 'attendee', got %q", fetched.Role)
	}
	if !fetched.IsActive {
		t.Errorf("Expected default IsActive to be true")
	}
	if fetched.Email != "" {
		t.Errorf("Expected empty string for absent email, got %q", fetched.Email)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user1 := &models.User{
		FullName:     "User One",
		Username:     "duplicate_user",
		PasswordHash: "hash1",
	}
	if err := CreateUser(user1); err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &models.User{
		FullName:     "User Two",
		Username:     "duplicate_user",
		PasswordHash: "hash2",
	}
	err := CreateUser(user2)
	if err == nil {
		t.Fatalf("Expected error for duplicate username, got nil")
	}

	if !strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
		t.Errorf("Expected UNIQUE constraint error on username, got %v", err)
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user1 := &models.User{
		FullName:     "User One",
		Username:     "user_email_1",
		Email:        "shared@example.com",
		PasswordHash: "hash1",
	}
	if err := CreateUser(user1); err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &models.User{
		FullName:     "User Two",
		Username:     "user_email_2",
		Email:        "shared@example.com",
		PasswordHash: "hash2",
	}
	err := CreateUser(user2)
	if err == nil {
		t.Fatalf("Expected error for duplicate email, got nil")
	}

	if !strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
		t.Errorf("Expected UNIQUE constraint error on email, got %v", err)
	}
}

func TestCreateUser_MultipleEmptyEmailsAllowed(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user1 := &models.User{
		FullName:     "No Email One",
		Username:     "no_email_1",
		Email:        "",
		PasswordHash: "hash1",
	}
	if err := CreateUser(user1); err != nil {
		t.Fatalf("Failed to create user1 without email: %v", err)
	}

	user2 := &models.User{
		FullName:     "No Email Two",
		Username:     "no_email_2",
		Email:        "",
		PasswordHash: "hash2",
	}
	if err := CreateUser(user2); err != nil {
		t.Fatalf("Failed to create user2 without email (empty string should not collide): %v", err)
	}
}

func TestExistingUsersRemainAccessible(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users inserted in setupTestDB: alice, bob, charlie
	alice, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("Failed to get seed user alice: %v", err)
	}
	if alice.FullName != "Alice Test" {
		t.Errorf("Expected 'Alice Test', got %q", alice.FullName)
	}

	bob, err := GetUserByID(2)
	if err != nil {
		t.Fatalf("Failed to get seed user bob by ID: %v", err)
	}
	if bob.Username != "bob" {
		t.Errorf("Expected 'bob', got %q", bob.Username)
	}
}

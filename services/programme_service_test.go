package services_test

import (
	"testing"

	"convention-management-system/models"
	"convention-management-system/services"
)

func TestExtractYouTubeVideoID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:      "Raw 11-char ID",
			input:     "dQw4w9WgXcQ",
			expected:  "dQw4w9WgXcQ",
			expectErr: false,
		},
		{
			name:      "Standard watch URL",
			input:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected:  "dQw4w9WgXcQ",
			expectErr: false,
		},
		{
			name:      "Watch URL with extra query params",
			input:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s&feature=emb_title",
			expected:  "dQw4w9WgXcQ",
			expectErr: false,
		},
		{
			name:      "Short youtu.be URL",
			input:     "https://youtu.be/dQw4w9WgXcQ?si=AbCdEfGhIjKlMnOp",
			expected:  "dQw4w9WgXcQ",
			expectErr: false,
		},
		{
			name:      "YouTube Live URL",
			input:     "https://www.youtube.com/live/dQw4w9WgXcQ?feature=share",
			expected:  "dQw4w9WgXcQ",
			expectErr: false,
		},
		{
			name:      "YouTube Embed URL",
			input:     "https://www.youtube.com/embed/dQw4w9WgXcQ",
			expected:  "dQw4w9WgXcQ",
			expectErr: false,
		},
		{
			name:      "Invalid length",
			input:     "tooshort",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "Invalid characters",
			input:     "https://www.youtube.com/watch?v=dQw4w9<script>",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "Empty input",
			input:     "   ",
			expected:  "",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := services.ExtractYouTubeVideoID(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil (extracted: %s)", tc.input, actual)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input %q: %v", tc.input, err)
				}
				if actual != tc.expected {
					t.Fatalf("expected %q, got %q", tc.expected, actual)
				}
			}
		})
	}
}

func TestCanUserViewProgramme(t *testing.T) {
	publicProg := &models.Programme{
		ID:         1,
		Visibility: models.ProgrammeVisibilityPublic,
	}

	membersOnlyProg := &models.Programme{
		ID:         2,
		Visibility: models.ProgrammeVisibilityMembersOnly,
	}

	// 1. Public programme is viewable by all
	if !services.CanUserViewProgramme(publicProg, 0, false) {
		t.Errorf("expected anonymous user to be able to view public programme")
	}
	if !services.CanUserViewProgramme(publicProg, 42, false) {
		t.Errorf("expected member to be able to view public programme")
	}
	if !services.CanUserViewProgramme(publicProg, 0, true) {
		t.Errorf("expected admin to be able to view public programme")
	}

	// 2. Members-only programme denies anonymous
	if services.CanUserViewProgramme(membersOnlyProg, 0, false) {
		t.Errorf("expected anonymous user to be blocked from viewing members-only programme")
	}

	// 3. Members-only programme allows logged-in member
	if !services.CanUserViewProgramme(membersOnlyProg, 42, false) {
		t.Errorf("expected member (ID 42) to be able to view members-only programme")
	}

	// 4. Members-only programme allows admin
	if !services.CanUserViewProgramme(membersOnlyProg, 0, true) {
		t.Errorf("expected admin to be able to view members-only programme")
	}
}

func TestCheckChatRateLimit(t *testing.T) {
	programmeID := 99999
	userID := 88888

	// First attempt should be allowed
	allowed, msg := services.CheckChatRateLimit(programmeID, userID)
	if !allowed {
		t.Fatalf("expected first chat attempt to be allowed, got error: %s", msg)
	}

	// Simulate a recent message by recording timestamp
	services.RecordChatMessageTime(programmeID, userID)

	// Immediate second attempt within 2 seconds must be blocked
	allowedSecond, warning := services.CheckChatRateLimit(programmeID, userID)
	if allowedSecond {
		t.Errorf("expected second immediate message to be blocked by rate limiter")
	}
	if warning == "" {
		t.Errorf("expected warning message from rate limiter")
	}
}

func TestProgrammeHelperMethods(t *testing.T) {
	p := &models.Programme{
		ID:                 10,
		Title:              "Youth Revival",
		Category:           models.ProgrammeCategoryYouthFellowship,
		YouTubeVideoID:     "dQw4w9WgXcQ",
		RecordingAvailable: true,
		RecordingVideoID:   "aBcDeFgHiJk",
		Status:             models.ProgrammeStatusLive,
	}

	if !p.IsLive() {
		t.Errorf("expected IsLive() to be true")
	}

	if p.EmbedURL() != "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0&modestbranding=1" {
		t.Errorf("unexpected live embed URL: %s", p.EmbedURL())
	}

	// Once ended with recording available, EmbedURL points to recording video
	p.Status = models.ProgrammeStatusEnded
	if !p.IsEnded() {
		t.Errorf("expected IsEnded() to be true")
	}
	if p.EmbedURL() != "https://www.youtube-nocookie.com/embed/aBcDeFgHiJk?rel=0&modestbranding=1" {
		t.Errorf("unexpected ended embed URL: %s", p.EmbedURL())
	}
}

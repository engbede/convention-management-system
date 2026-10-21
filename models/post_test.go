package models_test

import (
	"testing"

	"convention-management-system/models"
)

func TestPostVideoMimeType(t *testing.T) {
	tests := []struct {
		videoPath string
		expected  string
	}{
		{"posts/sample.mp4", "video/mp4"},
		{"posts/sample.webm", "video/webm"},
		{"posts/sample.mov", "video/quicktime"},
		{"posts/sample.mkv", "video/x-matroska"},
		{"posts/sample.avi", "video/x-msvideo"},
		{"posts/sample.mp4?t=123", "video/mp4"},
		{"", "video/mp4"},
	}

	for _, tc := range tests {
		p := models.Post{Video: tc.videoPath}
		if got := p.VideoMimeType(); got != tc.expected {
			t.Errorf("VideoMimeType() for %q = %q; want %q", tc.videoPath, got, tc.expected)
		}
	}
}

func TestPostIsWebPlayableVideo(t *testing.T) {
	tests := []struct {
		videoPath string
		expected  bool
	}{
		{"posts/sample.mp4", true},
		{"posts/sample.webm", true},
		{"posts/sample.mov", true},
		{"posts/sample.mkv", false},
		{"posts/sample.avi", false},
		{"", true},
	}

	for _, tc := range tests {
		p := models.Post{Video: tc.videoPath}
		if got := p.IsWebPlayableVideo(); got != tc.expected {
			t.Errorf("IsWebPlayableVideo() for %q = %v; want %v", tc.videoPath, got, tc.expected)
		}
	}
}

func TestPostVideoURL(t *testing.T) {
	t.Run("Empty video returns empty", func(t *testing.T) {
		p := models.Post{}
		if p.VideoURL() != "" {
			t.Errorf("Expected empty URL, got %s", p.VideoURL())
		}
	})

	t.Run("Local relative path gets prefixed", func(t *testing.T) {
		p := models.Post{Video: "posts/abc.mp4"}
		if got := p.VideoURL(); got != "/uploads/posts/abc.mp4" {
			t.Errorf("Expected /uploads/posts/abc.mp4, got %s", got)
		}
	})

	t.Run("Absolute HTTP URL preserved", func(t *testing.T) {
		p := models.Post{Video: "https://cdn.example.com/stream.mp4"}
		if got := p.VideoURL(); got != "https://cdn.example.com/stream.mp4" {
			t.Errorf("Expected unchanged HTTP URL, got %s", got)
		}
	})
}

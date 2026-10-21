package models

import (
	"testing"
	"time"
)

func TestCategorizeChronologicalGroupAt(t *testing.T) {
	// Reference now: Wednesday, October 21, 2026, 15:30:00 UTC
	now := time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		target   time.Time
		expected string
	}{
		{
			name:     "Zero time falls into Older",
			target:   time.Time{},
			expected: "Older",
		},
		{
			name:     "Today earlier in the day",
			target:   time.Date(2026, time.October, 21, 9, 0, 0, 0, time.UTC),
			expected: "Today",
		},
		{
			name:     "Today at start of day (midnight)",
			target:   time.Date(2026, time.October, 21, 0, 0, 1, 0, time.UTC),
			expected: "Today",
		},
		{
			name:     "Yesterday late evening",
			target:   time.Date(2026, time.October, 20, 23, 59, 59, 0, time.UTC),
			expected: "Yesterday",
		},
		{
			name:     "Yesterday early morning",
			target:   time.Date(2026, time.October, 20, 1, 0, 0, 0, time.UTC),
			expected: "Yesterday",
		},
		{
			name:     "Earlier this week (Monday Oct 19)",
			target:   time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC),
			expected: "Earlier This Week",
		},
		{
			name:     "Last week (Sunday Oct 18)",
			target:   time.Date(2026, time.October, 18, 18, 0, 0, 0, time.UTC),
			expected: "Last Week",
		},
		{
			name:     "Last week (Monday Oct 12)",
			target:   time.Date(2026, time.October, 12, 10, 0, 0, 0, time.UTC),
			expected: "Last Week",
		},
		{
			name:     "Earlier this month (Oct 5)",
			target:   time.Date(2026, time.October, 5, 14, 0, 0, 0, time.UTC),
			expected: "Earlier This Month",
		},
		{
			name:     "Older (September 2026)",
			target:   time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC),
			expected: "Older",
		},
		{
			name:     "Older (previous year 2025)",
			target:   time.Date(2025, time.December, 15, 8, 0, 0, 0, time.UTC),
			expected: "Older",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CategorizeChronologicalGroupAt(tc.target, now)
			if got != tc.expected {
				t.Errorf("CategorizeChronologicalGroupAt(%v) = %q; expected %q", tc.target, got, tc.expected)
			}
		})
	}
}

func TestFormatRelativeTimeAt(t *testing.T) {
	now := time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		target   time.Time
		expected string
	}{
		{
			name:     "Zero time returns empty",
			target:   time.Time{},
			expected: "",
		},
		{
			name:     "Just now (20 seconds ago)",
			target:   now.Add(-20 * time.Second),
			expected: "Just now",
		},
		{
			name:     "1 minute ago",
			target:   now.Add(-1 * time.Minute),
			expected: "1 minute ago",
		},
		{
			name:     "5 minutes ago",
			target:   now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			target:   now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "3 hours ago",
			target:   now.Add(-3 * time.Hour),
			expected: "3 hours ago",
		},
		{
			name:     "Yesterday",
			target:   time.Date(2026, time.October, 20, 10, 0, 0, 0, time.UTC),
			expected: "Yesterday",
		},
		{
			name:     "2 days ago",
			target:   time.Date(2026, time.October, 19, 10, 0, 0, 0, time.UTC),
			expected: "2 days ago",
		},
		{
			name:     "1 week ago",
			target:   time.Date(2026, time.October, 14, 10, 0, 0, 0, time.UTC),
			expected: "1 week ago",
		},
		{
			name:     "2 weeks ago",
			target:   time.Date(2026, time.October, 7, 10, 0, 0, 0, time.UTC),
			expected: "2 weeks ago",
		},
		{
			name:     "1 month ago (September)",
			target:   time.Date(2026, time.September, 15, 10, 0, 0, 0, time.UTC),
			expected: "1 month ago",
		},
		{
			name:     "Older date in same year",
			target:   time.Date(2026, time.March, 14, 10, 0, 0, 0, time.UTC),
			expected: "Mar 14",
		},
		{
			name:     "Older date in prior year",
			target:   time.Date(2025, time.December, 25, 10, 0, 0, 0, time.UTC),
			expected: "Dec 25, 2025",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatRelativeTimeAt(tc.target, now)
			if got != tc.expected {
				t.Errorf("FormatRelativeTimeAt(%v) = %q; expected %q", tc.target, got, tc.expected)
			}
		})
	}
}

func TestFormatDateSeparatorAt(t *testing.T) {
	now := time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		target   time.Time
		expected string
	}{
		{
			name:     "Today",
			target:   time.Date(2026, time.October, 21, 8, 15, 0, 0, time.UTC),
			expected: "Today",
		},
		{
			name:     "Yesterday",
			target:   time.Date(2026, time.October, 20, 22, 0, 0, 0, time.UTC),
			expected: "Yesterday",
		},
		{
			name:     "Specific weekday same year (Monday, 19 October)",
			target:   time.Date(2026, time.October, 19, 10, 0, 0, 0, time.UTC),
			expected: "Monday, 19 October",
		},
		{
			name:     "Specific weekday different year",
			target:   time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC),
			expected: "Monday, 1 December 2025",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatDateSeparatorAt(tc.target, now)
			if got != tc.expected {
				t.Errorf("FormatDateSeparatorAt(%v) = %q; expected %q", tc.target, got, tc.expected)
			}
		})
	}
}

func TestGroupConversationsChronologicallyAt(t *testing.T) {
	now := time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)

	items := []ConversationItem{
		{ID: 1, Title: "Today Conv 1", LastMessageTime: now.Add(-10 * time.Minute)},
		{ID: 2, Title: "Today Conv 2", LastMessageTime: now.Add(-2 * time.Hour)},
		{ID: 3, Title: "Yesterday Conv", LastMessageTime: time.Date(2026, time.October, 20, 14, 0, 0, 0, time.UTC)},
		{ID: 4, Title: "Earlier Week Conv", LastMessageTime: time.Date(2026, time.October, 19, 11, 0, 0, 0, time.UTC)},
		{ID: 5, Title: "Older Conv", LastMessageTime: time.Date(2026, time.July, 1, 10, 0, 0, 0, time.UTC)},
	}

	groups := GroupConversationsChronologicallyAt(items, now)

	// Verify only groups with conversations are present
	expectedHeadings := []string{"Today", "Yesterday", "Earlier This Week", "Older"}
	if len(groups) != len(expectedHeadings) {
		t.Fatalf("Expected %d groups, got %d", len(expectedHeadings), len(groups))
	}

	for i, expectedH := range expectedHeadings {
		if groups[i].Heading != expectedH {
			t.Errorf("Group[%d] heading = %q; expected %q", i, groups[i].Heading, expectedH)
		}
	}

	// Verify items inside Today group
	todayGroup := groups[0]
	if len(todayGroup.Conversations) != 2 {
		t.Fatalf("Expected 2 conversations in Today group, got %d", len(todayGroup.Conversations))
	}
	if todayGroup.Conversations[0].ID != 1 || todayGroup.Conversations[1].ID != 2 {
		t.Errorf("Expected Today conversations order [1, 2], got [%d, %d]",
			todayGroup.Conversations[0].ID, todayGroup.Conversations[1].ID)
	}
}

func TestMessageFormattedTimeAndDateKey(t *testing.T) {
	msgTime := time.Date(2026, time.October, 21, 14, 42, 0, 0, time.UTC)
	msg := Message{
		ID:        42,
		CreatedAt: msgTime,
	}

	timeStr := msg.FormattedTime()
	if timeStr != "2:42 PM" {
		t.Errorf("msg.FormattedTime() = %q; expected %q", timeStr, "2:42 PM")
	}

	dateKey := msg.DateKey()
	if dateKey != "2026-10-21" {
		t.Errorf("msg.DateKey() = %q; expected %q", dateKey, "2026-10-21")
	}
}


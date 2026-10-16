package repository

import (
	"fmt"
	"sync"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"
)

func TestAssignNextBibleStudyGroup_RangeAndDistribution(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := database.SeedConvention(); err != nil {
		t.Fatalf("SeedConvention failed: %v", err)
	}

	conv, err := GetActiveConvention()
	if err != nil {
		t.Fatalf("GetActiveConvention failed: %v", err)
	}

	// In an empty convention, the first assigned group should be 1
	firstGroup, err := AssignNextBibleStudyGroup(conv.ID)
	if err != nil {
		t.Fatalf("AssignNextBibleStudyGroup failed: %v", err)
	}
	if firstGroup != 1 {
		t.Errorf("expected first assigned group to be 1, got %d", firstGroup)
	}

	// Create 15 registrations assigned sequentially
	groupCounts := make(map[int]int)
	for i := 1; i <= 30; i++ {
		grp, err := AssignNextBibleStudyGroup(conv.ID)
		if err != nil {
			t.Fatalf("assignment %d failed: %v", i, err)
		}
		if grp < 1 || grp > 15 {
			t.Fatalf("assignment %d produced out of range group %d (must be 1..15)", i, grp)
		}

		// Insert registration with this group
		reg := models.Registration{
			ConventionID:    conv.ID,
			FullName:        "Test Attendee",
			Gender:          "Male",
			Phone:           fmt.Sprintf("08012345%03d", i),
			Circuit:         "Adoka",
			Membership:      "Member",
			BibleStudyGroup: grp,
		}
		if err := CreateRegistration(&reg); err != nil {
			t.Fatalf("CreateRegistration %d failed: %v", i, err)
		}

		groupCounts[grp]++
	}

	// After 30 assignments across 15 groups, each group must have exactly 2 members
	for g := 1; g <= 15; g++ {
		if count := groupCounts[g]; count != 2 {
			t.Errorf("expected group %d to have 2 members after 30 balanced assignments, got %d", g, count)
		}
	}
}

func TestAssignNextBibleStudyGroup_Concurrency(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := database.SeedConvention(); err != nil {
		t.Fatalf("SeedConvention failed: %v", err)
	}

	conv, err := GetActiveConvention()
	if err != nil {
		t.Fatalf("GetActiveConvention failed: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 30)

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			grp, err := AssignNextBibleStudyGroup(conv.ID)
			if err != nil {
				errCh <- err
				return
			}
			if grp < 1 || grp > 15 {
				t.Errorf("concurrent assignment produced invalid group %d", grp)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent assignment error: %v", err)
	}
}

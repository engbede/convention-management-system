package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

// CommunitySidebarData contains aggregated information for the community sidebar
type CommunitySidebarData struct {
	Stats          *models.CommunityStats
	UpcomingEvents []models.Programme
	PrayerRequests []models.CommunityPrayerRequest
	BibleStudy     *models.WeeklyBibleStudy
	Announcements  []models.Notice
	Publications   []models.Document
}

// GetCommunitySidebarData gathers all dynamic sidebar content
func GetCommunitySidebarData(viewerUserID int, isStaff bool) (*CommunitySidebarData, error) {
	data := &CommunitySidebarData{}

	// 1. Community Stats
	stats, err := repository.GetCommunityStats()
	if err == nil {
		data.Stats = stats
	} else {
		data.Stats = &models.CommunityStats{}
	}

	// 2. Upcoming Events
	events, err := repository.GetUpcomingProgrammes(4)
	if err == nil {
		data.UpcomingEvents = events
	}

	// 3. Prayer Requests with privacy scopes & intercession status
	prayerRequests, err := repository.GetCommunityPrayerRequests(viewerUserID, isStaff, 5)
	if err == nil {
		data.PrayerRequests = prayerRequests
	}

	// 4. Weekly Bible Study Topic
	data.BibleStudy = &models.WeeklyBibleStudy{
		Theme:            "Walking in Christian Integrity and Youth Excellence",
		ScripturePassage: "Daniel 1:8–17; 1 Timothy 4:12",
		MemoryVerse:      "1 Timothy 4:12 (KJV)",
		MemoryVerseText:  "\"Let no man despise thy youth; but be thou an example of the believers, in word, in conversation, in charity, in spirit, in faith, in purity.\"",
		DiscussionQuestions: []string{
			"How did Daniel maintain his conviction in a secular royal court?",
			"What daily practices protect a young believer's spiritual purity today?",
			"In what tangible ways can we be examples of faith in our fellowship?",
		},
		StudyGroupLink: "/chat",
	}

	// 5. Active Announcements & Notices
	notices, err := repository.GetActiveNotices()
	if err == nil {
		if len(notices) > 3 {
			notices = notices[:3]
		}
		data.Announcements = notices
	}

	// 6. Publications & Documents
	docs, err := repository.GetAllDocuments()
	if err == nil {
		if len(docs) > 3 {
			docs = docs[:3]
		}
		data.Publications = docs
	}

	return data, nil
}


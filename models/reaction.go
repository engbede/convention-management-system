package models

import "time"

type Reaction struct {
	ID int

	PostID int

	UserID int

	Reaction string

	CreatedAt time.Time
}

type ReactionSummary struct {
	Like int

	Love int

	Amen int

	Clap int

	Fire int

	Celebrate int

	Total int
}

// ReactionDetail describes a single curated reaction option.
type ReactionDetail struct {
	Type     string `json:"type"`
	Emoji    string `json:"emoji"`
	Label    string `json:"label"`
	Category string `json:"category"`
}

// ReactionCategoryGroup groups reactions for the category picker tabs.
type ReactionCategoryGroup struct {
	Name      string           `json:"name"`
	Reactions []ReactionDetail `json:"reactions"`
}

// ReactionUserDetail describes a user who reacted to a post.
type ReactionUserDetail struct {
	UserID       int    `json:"user_id"`
	FullName     string `json:"full_name"`
	Username     string `json:"username"`
	ProfilePhoto string `json:"profile_photo"`
	ReactionType string `json:"reaction_type"`
	Emoji        string `json:"emoji"`
}

// SupportedReactionCatalog lists all available curated stickers and emojis across the 8 categories.
var SupportedReactionCatalog = []ReactionDetail{
	// 1. Like & approval
	{Type: "like", Emoji: "👍", Label: "Like", Category: "Like & Approval"},
	{Type: "clap", Emoji: "👏", Label: "Applause", Category: "Like & Approval"},
	{Type: "ok", Emoji: "👌", Label: "Great", Category: "Like & Approval"},

	// 2. Love & affection
	{Type: "love", Emoji: "❤️", Label: "Love", Category: "Love & Affection"},
	{Type: "warmth", Emoji: "🥰", Label: "Warmth", Category: "Love & Affection"},
	{Type: "blessing", Emoji: "💖", Label: "Blessing", Category: "Love & Affection"},

	// 3. Joy & laughter
	{Type: "joy", Emoji: "😂", Label: "Joy", Category: "Joy & Laughter"},
	{Type: "smile", Emoji: "😊", Label: "Smile", Category: "Joy & Laughter"},
	{Type: "party", Emoji: "🥳", Label: "Celebrate", Category: "Joy & Laughter"},

	// 4. Celebration & applause
	{Type: "celebrate", Emoji: "🎉", Label: "Celebration", Category: "Celebration & Victory"},
	{Type: "star", Emoji: "🌟", Label: "Star", Category: "Celebration & Victory"},
	{Type: "victory", Emoji: "🏆", Label: "Victory", Category: "Celebration & Victory"},

	// 5. Prayer & encouragement
	{Type: "amen", Emoji: "🙏", Label: "Amen / Prayer", Category: "Prayer & Faith"},
	{Type: "peace", Emoji: "🕊️", Label: "Peace", Category: "Prayer & Faith"},
	{Type: "faith", Emoji: "✝️", Label: "Faith", Category: "Prayer & Faith"},

	// 6. Support & gratitude
	{Type: "praise", Emoji: "🙌", Label: "Praise", Category: "Support & Gratitude"},
	{Type: "fellowship", Emoji: "🤝", Label: "Fellowship", Category: "Support & Gratitude"},
	{Type: "gratitude", Emoji: "💐", Label: "Gratitude", Category: "Support & Gratitude"},

	// 7. Surprise & amazement
	{Type: "wow", Emoji: "😮", Label: "Wow", Category: "Surprise & Amazement"},
	{Type: "fire", Emoji: "🔥", Label: "On Fire", Category: "Surprise & Amazement"},
	{Type: "inspired", Emoji: "💡", Label: "Inspired", Category: "Surprise & Amazement"},

	// 8. Sadness & sympathy
	{Type: "sympathy", Emoji: "😢", Label: "Sympathy", Category: "Sadness & Sympathy"},
	{Type: "comfort", Emoji: "🫂", Label: "Comfort", Category: "Sadness & Sympathy"},
}

// IsValidReactionType checks whether a reaction key is supported.
func IsValidReactionType(r string) bool {
	for _, item := range SupportedReactionCatalog {
		if item.Type == r {
			return true
		}
	}
	return false
}

// GetReactionEmoji returns the display emoji for a reaction key.
func GetReactionEmoji(r string) string {
	for _, item := range SupportedReactionCatalog {
		if item.Type == r {
			return item.Emoji
		}
	}
	return "👍"
}

// GetReactionLabel returns the human-readable label for a reaction key.
func GetReactionLabel(r string) string {
	for _, item := range SupportedReactionCatalog {
		if item.Type == r {
			return item.Label
		}
	}
	return "Like"
}

// GetGroupedReactionCatalog returns reactions organized by category for easy rendering in templates.
func GetGroupedReactionCatalog() []ReactionCategoryGroup {
	catMap := make(map[string][]ReactionDetail)
	var catOrder []string

	for _, item := range SupportedReactionCatalog {
		if _, exists := catMap[item.Category]; !exists {
			catOrder = append(catOrder, item.Category)
		}
		catMap[item.Category] = append(catMap[item.Category], item)
	}

	var groups []ReactionCategoryGroup
	for _, name := range catOrder {
		groups = append(groups, ReactionCategoryGroup{
			Name:      name,
			Reactions: catMap[name],
		})
	}
	return groups
}

package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"strings"
)

func GetUserProfile(userID int) (*models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		email,
		phone,
		role,

		COALESCE(bio, ''),
		COALESCE(gender, ''),
		COALESCE(birth_date, ''),
		COALESCE(occupation, ''),

		COALESCE(location, ''),
		COALESCE(state, ''),
		COALESCE(country, ''),
		COALESCE(website, ''),

		COALESCE(profile_photo, ''),
		COALESCE(cover_photo, ''),

		COALESCE(church_name, ''),
		COALESCE(circuit, ''),
		COALESCE(local_church, ''),
		COALESCE(department, ''),
		COALESCE(position, ''),

		COALESCE(favorite_bible_verse, ''),
		COALESCE(life_verse, ''),
		COALESCE(salvation_testimony, ''),
		COALESCE(calling, ''),
		COALESCE(spiritual_gifts, ''),

		water_baptized,
		holy_spirit_baptized,

		COALESCE(favorite_preacher, ''),
		COALESCE(favorite_christian_book, ''),
		COALESCE(favorite_worship_song, ''),
		COALESCE(favorite_gospel_artist, ''),

		COALESCE(hobbies, ''),
		COALESCE(skills, ''),
		COALESCE(languages, ''),

		COALESCE(mission, ''),
		COALESCE(vision, ''),
		COALESCE(favorite_quote, ''),

		is_verified,
		is_active,

		followers,
		following,

		created_at,
		updated_at

	FROM users

	WHERE id = $1
	`

	var user models.User
	var waterBaptizedInt, holySpiritBaptizedInt int

	err := database.DB.QueryRow(query, userID).Scan(

		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.Role,

		// Basic Profile
		&user.Bio,
		&user.Gender,
		&user.BirthDate,
		&user.Occupation,

		&user.Location,
		&user.State,
		&user.Country,
		&user.Website,

		&user.ProfilePhoto,
		&user.CoverPhoto,

		// Church Information
		&user.ChurchName,
		&user.Circuit,
		&user.LocalChurch,
		&user.Department,
		&user.Position,

		// Spiritual Profile
		&user.FavoriteBibleVerse,
		&user.LifeVerse,
		&user.SalvationTestimony,
		&user.Calling,
		&user.SpiritualGifts,

		&waterBaptizedInt,
		&holySpiritBaptizedInt,

		// Interests
		&user.FavoritePreacher,
		&user.FavoriteChristianBook,
		&user.FavoriteWorshipSong,
		&user.FavoriteGospelArtist,

		&user.Hobbies,
		&user.Skills,
		&user.Languages,

		// Vision
		&user.Mission,
		&user.Vision,
		&user.FavoriteQuote,

		// Account Status
		&user.IsVerified,
		&user.IsActive,

		// Social
		&user.Followers,
		&user.Following,

		// Dates
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	user.WaterBaptized = (waterBaptizedInt == 1)
	user.HolySpiritBaptized = (holySpiritBaptizedInt == 1)

	// Generate initials
	parts := strings.Fields(user.FullName)

	switch len(parts) {

	case 0:
		user.Initials = "?"

	case 1:
		user.Initials = strings.ToUpper(parts[0][:1])

	default:
		user.Initials = strings.ToUpper(
			parts[0][:1] + parts[len(parts)-1][:1],
		)
	}

	return &user, nil
}

func UpdateUserProfile(user *models.User) error {

	query := `
	UPDATE users
	SET

	bio=$1,
	gender=$2,
	birth_date=$3,
	occupation=$4,

	location=$5,
	state=$6,
	country=$7,
	website=$8,

	church_name=$9,
	circuit=$10,
	local_church=$11,
	department=$12,
	position=$13,

	favorite_bible_verse=$14,
	life_verse=$15,
	salvation_testimony=$16,
	calling=$17,
	spiritual_gifts=$18,

	water_baptized=$19,
	holy_spirit_baptized=$20,

	favorite_preacher=$21,
	favorite_christian_book=$22,
	favorite_worship_song=$23,
	favorite_gospel_artist=$24,

	hobbies=$25,
	skills=$26,
	languages=$27,

	mission=$28,
	vision=$29,
	favorite_quote=$30,

	updated_at=CURRENT_TIMESTAMP

	WHERE id=$31
	`

	waterBaptizedInt := 0
	if user.WaterBaptized {
		waterBaptizedInt = 1
	}
	holySpiritBaptizedInt := 0
	if user.HolySpiritBaptized {
		holySpiritBaptizedInt = 1
	}

	_, err := database.DB.Exec(

		query,

		user.Bio,
		user.Gender,
		user.BirthDate,
		user.Occupation,

		user.Location,
		user.State,
		user.Country,
		user.Website,

		user.ChurchName,
		user.Circuit,
		user.LocalChurch,
		user.Department,
		user.Position,

		user.FavoriteBibleVerse,
		user.LifeVerse,
		user.SalvationTestimony,
		user.Calling,
		user.SpiritualGifts,

		waterBaptizedInt,
		holySpiritBaptizedInt,

		user.FavoritePreacher,
		user.FavoriteChristianBook,
		user.FavoriteWorshipSong,
		user.FavoriteGospelArtist,

		user.Hobbies,
		user.Skills,
		user.Languages,

		user.Mission,
		user.Vision,
		user.FavoriteQuote,

		user.ID,
	)

	return err
}

func UpdateProfilePhoto(
	userID int,
	filename string,
) error {

	query := `
	UPDATE users
	SET
		profile_photo = $1,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = $2
	`

	_, err := database.DB.Exec(
		query,
		filename,
		userID,
	)

	return err
}

func UpdateCoverPhoto(
	userID int,
	filename string,
) error {

	query := `
	UPDATE users
	SET
		cover_photo = $1,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = $2
	`

	_, err := database.DB.Exec(
		query,
		filename,
		userID,
	)

	return err
}

func GetProfilePhotos(userID int) (string, string, error) {

	var profilePhoto string
	var coverPhoto string

	query := `
	SELECT
		COALESCE(profile_photo, ''),
		COALESCE(cover_photo, '')
	FROM users
	WHERE id = $1
	`

	err := database.DB.QueryRow(query, userID).Scan(
		&profilePhoto,
		&coverPhoto,
	)

	if err != nil {
		return "", "", err
	}

	return profilePhoto, coverPhoto, nil
}

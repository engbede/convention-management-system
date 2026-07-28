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

	WHERE id = ?
	`

	var user models.User

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

		&user.WaterBaptized,
		&user.HolySpiritBaptized,

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

	bio=?,
	gender=?,
	birth_date=?,
	occupation=?,

	location=?,
	state=?,
	country=?,
	website=?,

	church_name=?,
	circuit=?,
	local_church=?,
	department=?,
	position=?,

	favorite_bible_verse=?,
	life_verse=?,
	salvation_testimony=?,
	calling=?,
	spiritual_gifts=?,

	water_baptized=?,
	holy_spirit_baptized=?,

	favorite_preacher=?,
	favorite_christian_book=?,
	favorite_worship_song=?,
	favorite_gospel_artist=?,

	hobbies=?,
	skills=?,
	languages=?,

	mission=?,
	vision=?,
	favorite_quote=?,

	updated_at=CURRENT_TIMESTAMP

	WHERE id=?
	`

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

		user.WaterBaptized,
		user.HolySpiritBaptized,

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
		profile_photo = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
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
		cover_photo = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
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
		profile_photo,
		cover_photo
	FROM users
	WHERE id = ?
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
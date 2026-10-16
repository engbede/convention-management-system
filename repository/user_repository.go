package repository

import (
	"database/sql"
	"strings"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateUser(user *models.User) error {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = time.Now()
	}
	if user.Role == "" {
		user.Role = "attendee"
	}
	user.IsActive = true

	var emailVal interface{} = user.Email
	if user.Email == "" {
		emailVal = nil
	}
	var phoneVal interface{} = user.Phone
	if user.Phone == "" {
		phoneVal = nil
	}

	query := `
INSERT INTO users (
	full_name,
	username,
	email,
	phone,

	password_hash,
	role,

	bio,
	gender,
	birth_date,
	occupation,

	country,
	state,
	location,
	website,

	profile_photo,
	cover_photo,

	church_name,
	circuit,
	local_church,
	department,
	position,

	favorite_bible_verse,
	life_verse,
	calling,
	spiritual_gifts,
	salvation_testimony,

	favorite_preacher,
	favorite_christian_book,
	favorite_worship_song,
	favorite_gospel_artist,

	skills,
	languages,
	hobbies,

	mission,
	vision,
	favorite_quote,

	water_baptized,
	holy_spirit_baptized,

	is_verified,
	is_active,

	followers,
	following,

	created_at,
	updated_at
)
VALUES (
	$1, $2, $3, $4,
	$5, $6,
	$7, $8, $9, $10,
	$11, $12, $13, $14,
	$15, $16,
	$17, $18, $19, $20, $21,
	$22, $23, $24, $25, $26,
	$27, $28, $29, $30,
	$31, $32, $33,
	$34, $35, $36,
	$37, $38,
	$39, $40,
	$41, $42,
	$43, $44
)
`

	res, err := database.DB.Exec(
		query,
		user.FullName,
		user.Username,
		emailVal,
		phoneVal,

		user.PasswordHash,
		user.Role,

		user.Bio,
		user.Gender,
		user.BirthDate,
		user.Occupation,

		user.Country,
		user.State,
		user.Location,
		user.Website,

		user.ProfilePhoto,
		user.CoverPhoto,

		user.ChurchName,
		user.Circuit,
		user.LocalChurch,
		user.Department,
		user.Position,

		user.FavoriteBibleVerse,
		user.LifeVerse,
		user.Calling,
		user.SpiritualGifts,
		user.SalvationTestimony,

		user.FavoritePreacher,
		user.FavoriteChristianBook,
		user.FavoriteWorshipSong,
		user.FavoriteGospelArtist,

		user.Skills,
		user.Languages,
		user.Hobbies,

		user.Mission,
		user.Vision,
		user.FavoriteQuote,

		user.WaterBaptized,
		user.HolySpiritBaptized,

		user.IsVerified,
		user.IsActive,

		user.Followers,
		user.Following,

		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if id, err := res.LastInsertId(); err == nil && id > 0 {
		user.ID = int(id)
	} else {
		_ = database.DB.QueryRow("SELECT id FROM users WHERE username = $1", user.Username).Scan(&user.ID)
	}

	return nil
}

func GetUserByUsername(username string) (*models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		COALESCE(email, ''),
		COALESCE(phone, ''),

		password_hash,
		COALESCE(role, 'attendee'),

		COALESCE(bio, ''),
		COALESCE(gender, ''),
		COALESCE(birth_date, ''),
		COALESCE(location, ''),

		COALESCE(website, ''),

		COALESCE(profile_photo, ''),
		COALESCE(cover_photo, ''),

		COALESCE(is_verified, 0),
		COALESCE(is_active, 1),

		COALESCE(followers, 0),
		COALESCE(following, 0),

		created_at,
		updated_at
	FROM users
	WHERE username = $1
	`

	var user models.User

	err := database.DB.QueryRow(query, username).Scan(

		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,

		&user.PasswordHash,
		&user.Role,

		&user.Bio,
		&user.Gender,
		&user.BirthDate,
		&user.Location,

		&user.Website,

		&user.ProfilePhoto,
		&user.CoverPhoto,

		&user.IsVerified,
		&user.IsActive,

		&user.Followers,
		&user.Following,

		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

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

func GetUserByID(id int) (models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		COALESCE(email, ''),
		COALESCE(phone, ''),
		COALESCE(role, 'attendee'),
		COALESCE(bio, ''),
		COALESCE(gender, ''),
		COALESCE(birth_date, ''),
		COALESCE(occupation, ''),
		COALESCE(country, ''),
		COALESCE(state, ''),
		COALESCE(location, ''),
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
		COALESCE(calling, ''),
		COALESCE(spiritual_gifts, ''),
		COALESCE(salvation_testimony, ''),
		COALESCE(favorite_preacher, ''),
		COALESCE(favorite_christian_book, ''),
		COALESCE(favorite_worship_song, ''),
		COALESCE(favorite_gospel_artist, ''),
		COALESCE(skills, ''),
		COALESCE(languages, ''),
		COALESCE(hobbies, ''),
		COALESCE(mission, ''),
		COALESCE(vision, ''),
		COALESCE(favorite_quote, ''),
		COALESCE(water_baptized, 0),
		COALESCE(holy_spirit_baptized, 0),
		COALESCE(is_verified, 0),
		COALESCE(is_active, 1),
		COALESCE(followers, 0),
		COALESCE(following, 0),
		created_at,
		updated_at
	FROM users
	WHERE id = $1
	`

	var user models.User

	err := database.DB.QueryRow(query, id).Scan(

		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.Role,

		&user.Bio,
		&user.Gender,
		&user.BirthDate,
		&user.Occupation,

		&user.Country,
		&user.State,
		&user.Location,
		&user.Website,

		&user.ProfilePhoto,
		&user.CoverPhoto,

		&user.ChurchName,
		&user.Circuit,
		&user.LocalChurch,
		&user.Department,
		&user.Position,

		&user.FavoriteBibleVerse,
		&user.LifeVerse,
		&user.Calling,
		&user.SpiritualGifts,
		&user.SalvationTestimony,

		&user.FavoritePreacher,
		&user.FavoriteChristianBook,
		&user.FavoriteWorshipSong,
		&user.FavoriteGospelArtist,

		&user.Skills,
		&user.Languages,
		&user.Hobbies,

		&user.Mission,
		&user.Vision,
		&user.FavoriteQuote,

		&user.WaterBaptized,
		&user.HolySpiritBaptized,

		&user.IsVerified,
		&user.IsActive,

		&user.Followers,
		&user.Following,

		&user.CreatedAt,
		&user.UpdatedAt,
	)

	return user, err
}

func GetUserByEmail(email string) (*models.User, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return nil, sql.ErrNoRows
	}

	query := `
	SELECT
		id,
		full_name,
		username,
		COALESCE(email, ''),
		COALESCE(phone, ''),

		password_hash,
		COALESCE(role, 'attendee'),

		COALESCE(bio, ''),
		COALESCE(gender, ''),
		COALESCE(birth_date, ''),
		COALESCE(location, ''),

		COALESCE(website, ''),

		COALESCE(profile_photo, ''),
		COALESCE(cover_photo, ''),

		COALESCE(is_verified, 0),
		COALESCE(is_active, 1),

		COALESCE(followers, 0),
		COALESCE(following, 0),

		created_at,
		updated_at
	FROM users
	WHERE LOWER(email) = LOWER($1)
	LIMIT 1
	`

	var user models.User
	err := database.DB.QueryRow(query, trimmed).Scan(
		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.Role,
		&user.Bio,
		&user.Gender,
		&user.BirthDate,
		&user.Location,
		&user.Website,
		&user.ProfilePhoto,
		&user.CoverPhoto,
		&user.IsVerified,
		&user.IsActive,
		&user.Followers,
		&user.Following,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

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

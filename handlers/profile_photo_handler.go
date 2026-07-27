package handlers

import (
	"net/http"
	"path/filepath"

	"convention-management-system/services"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

func UploadProfilePhoto(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("profile_photo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	oldProfilePhoto, _, err := services.GetProfilePhotos(userID)
	if err == nil && oldProfilePhoto != "" {

		utils.DeleteFile(
			filepath.Join(
				"uploads/profiles",
				oldProfilePhoto,
			),
		)

	}

	filename, err := utils.SaveUploadedFile(
		file,
		header,
		"uploads/profiles",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = services.UpdateProfilePhoto(userID, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func UploadCoverPhoto(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("cover_photo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, oldCoverPhoto, err := services.GetProfilePhotos(userID)
	if err == nil && oldCoverPhoto != "" {

		utils.DeleteFile(
			filepath.Join(
				"uploads/covers",
				oldCoverPhoto,
			),
		)

	}
	
	filename, err := utils.SaveUploadedFile(
		file,
		header,
		"uploads/covers",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = services.UpdateCoverPhoto(userID, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

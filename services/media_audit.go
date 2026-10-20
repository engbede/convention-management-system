package services

import (
	"database/sql"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"convention-management-system/database"
	"convention-management-system/utils"
)

// MediaAuditReport records the results of an auditable, non-destructive media scan and recovery.
type MediaAuditReport struct {
	TotalPostsAudited        int      `json:"total_posts_audited"`
	PostsWithMedia           int      `json:"posts_with_media"`
	TotalUsersAudited        int      `json:"total_users_audited"`
	UsersWithPhotos          int      `json:"users_with_photos"`
	FilesSynchronized        int      `json:"files_synchronized"`
	UnrecoverablePostImages  []string `json:"unrecoverable_post_images"`
	UnrecoverablePostVideos  []string `json:"unrecoverable_post_videos"`
	UnrecoverableUserPhotos  []string `json:"unrecoverable_user_photos"`
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// AuditAndRepairMedia non-destructively scans media records, restores files from fallback locations to durable storage,
// and identifies genuinely missing files without deleting or fabricating data.
func AuditAndRepairMedia() (*MediaAuditReport, error) {
	report := &MediaAuditReport{
		UnrecoverablePostImages: make([]string, 0),
		UnrecoverablePostVideos: make([]string, 0),
		UnrecoverableUserPhotos: make([]string, 0),
	}

	if database.DB == nil {
		return report, nil
	}

	baseDir := utils.GetUploadBaseDir()
	_ = os.MkdirAll(filepath.Join(baseDir, "posts"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "profiles"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "covers"), 0755)

	// 1. Audit Posts
	postRows, err := database.DB.Query(`SELECT id, COALESCE(image, ''), COALESCE(video, '') FROM posts ORDER BY id ASC`)
	if err == nil {
		defer postRows.Close()
		for postRows.Next() {
			var postID int
			var img, vid string
			if err := postRows.Scan(&postID, &img, &vid); err != nil {
				continue
			}
			report.TotalPostsAudited++

			if img != "" || vid != "" {
				report.PostsWithMedia++
			}

			// Audit Post Image
			if img != "" && !strings.HasPrefix(img, "http://") && !strings.HasPrefix(img, "https://") {
				cleanImg := strings.TrimPrefix(img, "/uploads/")
				cleanImg = strings.TrimPrefix(cleanImg, "/")
				targetPath := filepath.Join(baseDir, filepath.Clean("/"+cleanImg))

				if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
					// Check local fallback directory "uploads/..."
					localFallback := filepath.Join("uploads", filepath.Clean("/"+cleanImg))
					if _, lErr := os.Stat(localFallback); lErr == nil {
						if copyErr := copyFile(localFallback, targetPath); copyErr == nil {
							report.FilesSynchronized++
							log.Printf("[Media Audit] Synchronized post %d image from %s to durable path %s", postID, localFallback, targetPath)
							continue
						}
					}
					report.UnrecoverablePostImages = append(report.UnrecoverablePostImages, cleanImg)
				}
			}

			// Audit Post Video
			if vid != "" && !strings.HasPrefix(vid, "http://") && !strings.HasPrefix(vid, "https://") {
				cleanVid := strings.TrimPrefix(vid, "/uploads/")
				cleanVid = strings.TrimPrefix(cleanVid, "/")
				targetPath := filepath.Join(baseDir, filepath.Clean("/"+cleanVid))

				if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
					localFallback := filepath.Join("uploads", filepath.Clean("/"+cleanVid))
					if _, lErr := os.Stat(localFallback); lErr == nil {
						if copyErr := copyFile(localFallback, targetPath); copyErr == nil {
							report.FilesSynchronized++
							log.Printf("[Media Audit] Synchronized post %d video from %s to durable path %s", postID, localFallback, targetPath)
							continue
						}
					}
					report.UnrecoverablePostVideos = append(report.UnrecoverablePostVideos, cleanVid)
				}
			}
		}
	}

	// 2. Audit Users Profile Photos
	userRows, err := database.DB.Query(`SELECT id, username, COALESCE(profile_photo, '') FROM users ORDER BY id ASC`)
	if err == nil {
		defer userRows.Close()
		for userRows.Next() {
			var userID int
			var uname, photo string
			if err := userRows.Scan(&userID, &uname, &photo); err != nil {
				continue
			}
			report.TotalUsersAudited++

			if photo != "" {
				report.UsersWithPhotos++
				if !strings.HasPrefix(photo, "http://") && !strings.HasPrefix(photo, "https://") {
					cleanPhoto := strings.TrimPrefix(photo, "/uploads/profiles/")
					cleanPhoto = strings.TrimPrefix(cleanPhoto, "/uploads/")
					cleanPhoto = strings.TrimPrefix(cleanPhoto, "profiles/")
					cleanPhoto = strings.TrimPrefix(cleanPhoto, "/")

					targetPath := filepath.Join(baseDir, "profiles", cleanPhoto)
					if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
						// Check local fallback
						localFallback := filepath.Join("uploads", "profiles", cleanPhoto)
						if _, lErr := os.Stat(localFallback); lErr == nil {
							if copyErr := copyFile(localFallback, targetPath); copyErr == nil {
								report.FilesSynchronized++
								log.Printf("[Media Audit] Synchronized user %s (%d) profile photo to durable storage", uname, userID)
								continue
							}
						}
						report.UnrecoverableUserPhotos = append(report.UnrecoverableUserPhotos, cleanPhoto)
					}
				}
			}
		}
	}

	return report, nil
}

// AutoSyncEmbeddedMedia runs non-destructive synchronization at startup.
func AutoSyncEmbeddedMedia(db *sql.DB) {
	if _, err := AuditAndRepairMedia(); err != nil {
		log.Println("[Media Audit] Startup media audit encountered non-fatal error:", err)
	}
}


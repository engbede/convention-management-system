package services

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"convention-management-system/database"
	"convention-management-system/repository"
	"convention-management-system/utils"
)

func init() {
	utils.SetMediaStorageHooks(repository.SaveMediaBlob, repository.DeleteMediaBlob)
}

// SaveDurableMediaBlob persists media into PostgreSQL or SQLite durable storage.
func SaveDurableMediaBlob(key, contentType string, data []byte) error {
	return repository.SaveMediaBlob(key, contentType, data)
}

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
// 1. Registers the durable storage hooks in utils.
// 2. Synchronizes files from disk into PostgreSQL durable storage (so existing pictures become durable).
// 3. Restores missing files from PostgreSQL durable storage back to disk (recovering from ephemeral container restarts).
func AutoSyncEmbeddedMedia(db *sql.DB) {
	// Register persistence hooks
	utils.SetMediaStorageHooks(repository.SaveMediaBlob, repository.DeleteMediaBlob)

	if db == nil {
		return
	}

	baseDir := utils.GetUploadBaseDir()
	_ = os.MkdirAll(filepath.Join(baseDir, "posts"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "profiles"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "covers"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "chat"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "documents"), 0755)

	// Step A: Backup existing disk files to PostgreSQL durable store
	for _, folder := range []string{"profiles", "covers", "posts", "chat"} {
		dir := filepath.Join(baseDir, folder)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			mediaKey := folder + "/" + name
			filePath := filepath.Join(dir, name)

			// Skip files larger than max durable video size to prevent high memory usage during startup sync
			if fi, sErr := entry.Info(); sErr == nil && fi.Size() > utils.GetMaxDurableVideoSize() {
				continue
			}

			// Check if already in media_storage
			if _, _, err := repository.GetMediaBlob(mediaKey); err == nil {
				continue // already saved
			}

			// Read file and save to media_storage
			data, err := os.ReadFile(filePath)
			if err != nil || len(data) == 0 {
				continue
			}
			contentType := http.DetectContentType(data)
			if err := repository.SaveMediaBlob(mediaKey, contentType, data); err == nil {
				log.Printf("[Media Persistence] Backed up %s to database durable store", mediaKey)
			}
		}
	}

	// Step B: Restore missing files from PostgreSQL durable store to disk
	items, err := repository.ListMediaStorageKeys()
	if err == nil {
		for _, item := range items {
			targetPath := filepath.Join(baseDir, filepath.Clean("/"+item.MediaKey))
			if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
				data, _, err := repository.GetMediaBlob(item.MediaKey)
				if err == nil && len(data) > 0 {
					_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
					if wErr := os.WriteFile(targetPath, data, 0644); wErr == nil {
						log.Printf("[Media Persistence] Restored missing file %s from database to disk", item.MediaKey)
						// Also sync to local "uploads/" if baseDir is different
						if baseDir != "uploads" {
							localPath := filepath.Join("uploads", filepath.Clean("/"+item.MediaKey))
							_ = os.MkdirAll(filepath.Dir(localPath), 0755)
							_ = os.WriteFile(localPath, data, 0644)
						}
					}
				}
			}
		}
	}

	if _, err := AuditAndRepairMedia(); err != nil {
		log.Println("[Media Audit] Startup media audit encountered non-fatal error:", err)
	}
}

// CloudMigrationReport tracks media items migrated to Cloudflare R2 or S3.
type CloudMigrationReport struct {
	TotalInspected int      `json:"total_inspected"`
	AlreadyInCloud int      `json:"already_in_cloud"`
	MigratedCount  int      `json:"migrated_count"`
	FailedCount    int      `json:"failed_count"`
	Errors         []string `json:"errors"`
}

// MigrateMediaToCloudStorage safely and idempotently copies existing disk and DB media to Cloudflare R2/S3.
// Original local files and database records are strictly retained for rollback safety.
func MigrateMediaToCloudStorage() (*CloudMigrationReport, error) {
	report := &CloudMigrationReport{
		Errors: make([]string, 0),
	}

	if !utils.IsObjectStorageEnabled() {
		return report, errors.New("cannot migrate media: cloud object storage is not configured")
	}

	if err := utils.ValidateStorageConfig(); err != nil {
		return report, fmt.Errorf("invalid storage configuration: %w", err)
	}

	baseDir := utils.GetUploadBaseDir()

	// 1. Migrate items from PostgreSQL media_storage
	items, err := repository.ListMediaStorageKeys()
	if err == nil {
		for _, item := range items {
			report.TotalInspected++
			cleanKey := strings.TrimPrefix(item.MediaKey, "/")

			// Check if already in cloud
			if exists, _, _ := utils.CheckObjectExists(cleanKey); exists {
				report.AlreadyInCloud++
				continue
			}

			// First check if available on disk for streaming
			diskPath := filepath.Join(baseDir, filepath.Clean("/"+cleanKey))
			if f, oErr := os.Open(diskPath); oErr == nil {
				if fi, sErr := f.Stat(); sErr == nil {
					_, uErr := utils.UploadToObjectStorage(cleanKey, f, fi.Size(), item.ContentType)
					_ = f.Close()
					if uErr != nil {
						report.FailedCount++
						report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", cleanKey, uErr))
					} else {
						report.MigratedCount++
						log.Printf("[Media Migration] Migrated %s from disk to cloud storage", cleanKey)
					}
					continue
				}
				_ = f.Close()
			}

			// Fall back to reading from database blob
			data, cType, gErr := repository.GetMediaBlob(cleanKey)
			if gErr != nil || len(data) == 0 {
				report.FailedCount++
				report.Errors = append(report.Errors, fmt.Sprintf("%s: not found on disk or DB", cleanKey))
				continue
			}

			_, uErr := utils.UploadToObjectStorage(cleanKey, bytes.NewReader(data), int64(len(data)), cType)
			if uErr != nil {
				report.FailedCount++
				report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", cleanKey, uErr))
			} else {
				report.MigratedCount++
				log.Printf("[Media Migration] Migrated %s from database blob to cloud storage", cleanKey)
			}
		}
	}

	return report, nil
}


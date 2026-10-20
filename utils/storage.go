package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultAvatarSVG provides a clean fallback image when a profile photo is missing.
var DefaultAvatarSVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <circle cx="64" cy="64" r="64" fill="#198754"/>
  <circle cx="64" cy="48" r="22" fill="#ffffff" opacity="0.95"/>
  <path d="M24,112 C24,88 42,78 64,78 C86,78 104,88 104,112 Z" fill="#ffffff" opacity="0.95"/>
</svg>`)

// GetObjectStoragePublicBaseURL returns the configured public CDN or S3 base URL, if any.
func GetObjectStoragePublicBaseURL() string {
	for _, env := range []string{"S3_PUBLIC_URL", "MEDIA_BASE_URL", "CLOUDINARY_URL", "CDN_URL"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			return strings.TrimSuffix(val, "/")
		}
	}
	return ""
}

// S3Config holds connection parameters for S3-compatible object storage (AWS, Cloudflare R2, MinIO, Backblaze B2).
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
	PublicURL string
}

// GetS3Config reads S3 configuration from environment variables.
func GetS3Config() *S3Config {
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	if bucket == "" {
		bucket = strings.TrimSpace(os.Getenv("AWS_S3_BUCKET"))
	}
	if bucket == "" {
		return nil
	}

	region := strings.TrimSpace(os.Getenv("S3_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if region == "" {
		region = "auto"
	}

	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", region)
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	accessKey := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
	if accessKey == "" {
		accessKey = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	}

	secretKey := strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
	if secretKey == "" {
		secretKey = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	}

	if accessKey == "" || secretKey == "" {
		return nil
	}

	publicURL := GetObjectStoragePublicBaseURL()

	return &S3Config{
		Bucket:    bucket,
		Region:    region,
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		PublicURL: publicURL,
	}
}

// IsObjectStorageEnabled returns true if S3 credentials and bucket are configured.
func IsObjectStorageEnabled() bool {
	return GetS3Config() != nil
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// UploadToObjectStorage stores an object in S3-compatible cloud storage using AWS SigV4.
func UploadToObjectStorage(key string, reader io.Reader, size int64, contentType string) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, cleanKey)

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed reading object body for S3 upload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)

	// AWS SigV4 signing
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := sha256Hex(bodyBytes)

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	host := req.URL.Host
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", contentType, host, payloadHash, amzDate)
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	canonicalReq := fmt.Sprintf("%s\n/%s/%s\n\n%s\n%s\n%s", http.MethodPut, cfg.Bucket, cleanKey, canonicalHeaders, signedHeaders, payloadHash)
	canonicalHash := sha256Hex([]byte(canonicalReq))

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, canonicalHash)

	kDate := hmacSHA256([]byte("AWS4"+cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", cfg.AccessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("S3 upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("S3 upload returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if cfg.PublicURL != "" {
		return fmt.Sprintf("%s/%s", cfg.PublicURL, cleanKey), nil
	}
	return reqURL, nil
}

// DeleteFromObjectStorage deletes an object from S3-compatible cloud storage using AWS SigV4.
func DeleteFromObjectStorage(key string) error {
	cfg := GetS3Config()
	if cfg == nil {
		return nil
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, cleanKey)

	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := sha256Hex([]byte(""))

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	host := req.URL.Host
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalReq := fmt.Sprintf("%s\n/%s/%s\n\n%s\n%s\n%s", http.MethodDelete, cfg.Bucket, cleanKey, canonicalHeaders, signedHeaders, payloadHash)
	canonicalHash := sha256Hex([]byte(canonicalReq))

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, canonicalHash)

	kDate := hmacSHA256([]byte("AWS4"+cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", cfg.AccessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

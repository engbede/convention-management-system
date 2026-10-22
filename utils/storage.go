package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultAvatarSVG provides a clean fallback image when a profile photo is missing.
var DefaultAvatarSVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <circle cx="64" cy="64" r="64" fill="#198754"/>
  <circle cx="64" cy="48" r="22" fill="#ffffff" opacity="0.95"/>
  <path d="M24,112 C24,88 42,78 64,78 C86,78 104,88 104,112 Z" fill="#ffffff" opacity="0.95"/>
</svg>`)

// DefaultPostImageSVG provides a clean fallback image when a historical post image is missing.
var DefaultPostImageSVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 400" width="100%" height="100%">
  <rect width="600" height="400" fill="#f8f9fa"/>
  <rect x="150" y="80" width="300" height="200" rx="12" fill="#e9ecef" stroke="#dee2e6" stroke-width="2"/>
  <circle cx="210" cy="140" r="22" fill="#adb5bd"/>
  <path d="M160,260 L240,180 L310,240 L360,200 L440,260 Z" fill="#6c757d" opacity="0.6"/>
  <text x="300" y="320" font-family="-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif" font-size="14" fill="#6c757d" text-anchor="middle">Community Media Unavailable</text>
</svg>`)

// GetObjectStoragePublicBaseURL returns the configured public CDN or S3/R2 base URL, if any.
func GetObjectStoragePublicBaseURL() string {
	for _, env := range []string{"S3_PUBLIC_URL", "MEDIA_BASE_URL", "CLOUDINARY_URL", "CDN_URL"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			return strings.TrimSuffix(val, "/")
		}
	}
	return ""
}

// S3Config holds connection parameters for S3-compatible object storage (Cloudflare R2, AWS, MinIO, Backblaze B2).
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
	PublicURL string
}

// GetS3Config reads S3/R2 configuration from environment variables.
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

// IsCloudflareR2 returns true if the configured endpoint points to Cloudflare R2.
func IsCloudflareR2() bool {
	cfg := GetS3Config()
	if cfg == nil {
		return false
	}
	return strings.Contains(strings.ToLower(cfg.Endpoint), "r2.cloudflarestorage.com")
}

// ValidateStorageConfig verifies that storage environment variables form a valid configuration
// without exposing sensitive credentials in error messages.
func ValidateStorageConfig() error {
	cfg := GetS3Config()
	if cfg == nil {
		return errors.New("object storage is not configured: missing bucket or credentials")
	}

	if cfg.Bucket == "" {
		return errors.New("storage bucket name cannot be empty")
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("invalid storage endpoint URL: must be valid absolute URL (e.g. https://<account_id>.r2.cloudflarestorage.com)")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("storage endpoint scheme must be http or https")
	}

	if len(cfg.AccessKey) < 4 {
		return errors.New("storage access key is too short or invalid")
	}
	if len(cfg.SecretKey) < 8 {
		return errors.New("storage secret key is too short or invalid")
	}

	return nil
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

func buildCanonicalQueryFromValues(vals url.Values) string {
	if len(vals) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, k := range keys {
		escapedK := url.QueryEscape(k)
		vList := vals[k]
		sort.Strings(vList)
		for _, val := range vList {
			if builder.Len() > 0 {
				builder.WriteString("&")
			}
			builder.WriteString(escapedK)
			builder.WriteString("=")
			builder.WriteString(url.QueryEscape(val))
		}
	}
	return builder.String()
}

func buildCanonicalQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	parsed, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	return buildCanonicalQueryFromValues(parsed)
}

func signS3Request(req *http.Request, cfg *S3Config, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	host := req.URL.Host

	// Collect headers to sign
	headersToSign := make(map[string]string)
	headersToSign["host"] = host
	headersToSign["x-amz-date"] = amzDate
	headersToSign["x-amz-content-sha256"] = payloadHash

	if ct := req.Header.Get("Content-Type"); ct != "" {
		headersToSign["content-type"] = ct
	}

	// Lexicographically sort signed header names (mandatory for SigV4 compliance)
	sortedKeys := make([]string, 0, len(headersToSign))
	for k := range headersToSign {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var canonicalHeaders strings.Builder
	for _, k := range sortedKeys {
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", k, headersToSign[k]))
	}
	signedHeadersStr := strings.Join(sortedKeys, ";")

	canonicalQuery := buildCanonicalQuery(req.URL.RawQuery)
	canonicalReq := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", req.Method, req.URL.Path, canonicalQuery, canonicalHeaders.String(), signedHeadersStr, payloadHash)
	canonicalHash := sha256Hex([]byte(canonicalReq))

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, canonicalHash)

	kDate := hmacSHA256([]byte("AWS4"+cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", cfg.AccessKey, credentialScope, signedHeadersStr, signature)
	req.Header.Set("Authorization", authHeader)
}

// UploadToObjectStorage stores an object in S3/R2-compatible storage using AWS SigV4.
// For files of known size, it streams directly from reader using UNSIGNED-PAYLOAD to avoid buffering in memory.
func UploadToObjectStorage(key string, reader io.Reader, size int64, contentType string) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3/R2 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, cleanKey)

	req, err := http.NewRequest(http.MethodPut, reqURL, reader)
	if err != nil {
		return "", fmt.Errorf("failed creating S3 upload request: %w", err)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)

	if size >= 0 {
		req.ContentLength = size
	}

	// Cloudflare R2 and AWS S3 both support UNSIGNED-PAYLOAD over TLS for streaming uploads without in-memory buffering
	signS3Request(req, cfg, "UNSIGNED-PAYLOAD")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("S3 upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("S3 upload returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if cfg.PublicURL != "" {
		return fmt.Sprintf("%s/%s", cfg.PublicURL, cleanKey), nil
	}
	return reqURL, nil
}

// CheckObjectExists performs a HEAD request to verify if an object exists in object storage and returns its size.
func CheckObjectExists(key string) (bool, int64, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return false, 0, errors.New("S3/R2 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, cleanKey)

	req, err := http.NewRequest(http.MethodHead, reqURL, nil)
	if err != nil {
		return false, 0, err
	}

	signS3Request(req, cfg, sha256Hex([]byte("")))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, fmt.Errorf("HEAD object request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, resp.ContentLength, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, 0, nil
	}

	return false, 0, fmt.Errorf("HEAD object returned status %d", resp.StatusCode)
}

// DeleteFromObjectStorage deletes an object from S3/R2-compatible storage using AWS SigV4.
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

	signS3Request(req, cfg, sha256Hex([]byte("")))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// S3CompletedPart represents a single uploaded part in an S3 multipart upload.
type S3CompletedPart struct {
	PartNumber int    `xml:"PartNumber" json:"part_number"`
	ETag       string `xml:"ETag" json:"etag"`
}

type completeMultipartUploadReq struct {
	XMLName xml.Name          `xml:"CompleteMultipartUpload"`
	Parts   []S3CompletedPart `xml:"Part"`
}

type initiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

// InitiateS3MultipartUpload begins an S3/R2 multipart upload and returns the upload ID.
// Unsupported S3 ACL headers (like x-amz-acl) are omitted for Cloudflare R2 compatibility.
func InitiateS3MultipartUpload(key, contentType string) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3/R2 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s?uploads", cfg.Endpoint, cfg.Bucket, cleanKey)

	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return "", err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)

	signS3Request(req, cfg, sha256Hex([]byte("")))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("initiate multipart upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("initiate multipart upload returned status %d: %s", resp.StatusCode, string(body))
	}

	var res initiateMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to parse initiate multipart XML: %w", err)
	}

	if res.UploadId == "" {
		return "", errors.New("empty upload ID returned by S3 provider")
	}

	return res.UploadId, nil
}

// UploadS3Part uploads a single chunk/part to an ongoing S3/R2 multipart upload.
// Streams directly from reader using UNSIGNED-PAYLOAD with bounded memory.
func UploadS3Part(key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3/R2 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s?partNumber=%d&uploadId=%s", cfg.Endpoint, cfg.Bucket, cleanKey, partNumber, url.QueryEscape(uploadID))

	req, err := http.NewRequest(http.MethodPut, reqURL, reader)
	if err != nil {
		return "", err
	}

	if size >= 0 {
		req.ContentLength = size
	}

	signS3Request(req, cfg, "UNSIGNED-PAYLOAD")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload part %d failed: %w", partNumber, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("upload part %d returned status %d: %s", partNumber, resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	if etag == "" {
		return "", fmt.Errorf("upload part %d succeeded but missing ETag header", partNumber)
	}

	return etag, nil
}

// CompleteS3MultipartUpload finalizes an S3/R2 multipart upload after all parts are uploaded.
func CompleteS3MultipartUpload(key, uploadID string, parts []S3CompletedPart) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3/R2 object storage is not configured")
	}

	// Sort parts by PartNumber ascending (required by S3/R2 API)
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	xmlBody, err := xml.Marshal(completeMultipartUploadReq{Parts: parts})
	if err != nil {
		return "", fmt.Errorf("failed to marshal complete multipart XML: %w", err)
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s?uploadId=%s", cfg.Endpoint, cfg.Bucket, cleanKey, url.QueryEscape(uploadID))

	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(xmlBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/xml")

	signS3Request(req, cfg, sha256Hex(xmlBody))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("complete multipart upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("complete multipart upload returned status %d: %s", resp.StatusCode, string(body))
	}

	if cfg.PublicURL != "" {
		return fmt.Sprintf("%s/%s", cfg.PublicURL, cleanKey), nil
	}
	return fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, cleanKey), nil
}

// AbortS3MultipartUpload cancels an active S3/R2 multipart upload and cleans up stored parts.
func AbortS3MultipartUpload(key, uploadID string) error {
	cfg := GetS3Config()
	if cfg == nil {
		return nil
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s?uploadId=%s", cfg.Endpoint, cfg.Bucket, cleanKey, url.QueryEscape(uploadID))

	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}

	signS3Request(req, cfg, sha256Hex([]byte("")))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GeneratePresignedGetURL creates an authenticated presigned GET URL with SigV4 query parameters.
// It signs directly against the S3/R2 API endpoint with explicit expiration.
func GeneratePresignedGetURL(key string, expire time.Duration) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3/R2 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, cfg.Region)

	if expire <= 0 {
		expire = 15 * time.Minute
	}
	if expire > 7*24*time.Hour {
		expire = 7 * 24 * time.Hour
	}
	expiresSec := int(expire.Seconds())

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid S3 endpoint URL: %w", err)
	}
	host := u.Host

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", cfg.AccessKey, credentialScope))
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(expiresSec))
	q.Set("X-Amz-SignedHeaders", "host")

	canonicalQuery := buildCanonicalQueryFromValues(q)
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	canonicalURI := fmt.Sprintf("/%s/%s", cfg.Bucket, cleanKey)
	canonicalReq := fmt.Sprintf("GET\n%s\n%s\n%s\nhost\nUNSIGNED-PAYLOAD", canonicalURI, canonicalQuery, canonicalHeaders)
	canonicalHash := sha256Hex([]byte(canonicalReq))

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, canonicalHash)

	kDate := hmacSHA256([]byte("AWS4"+cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	return fmt.Sprintf("%s/%s/%s?%s&X-Amz-Signature=%s", cfg.Endpoint, cfg.Bucket, cleanKey, canonicalQuery, signature), nil
}

// GeneratePresignedPutURL creates an authenticated presigned PUT URL with SigV4 query parameters
// for direct browser-to-R2 uploads.
func GeneratePresignedPutURL(key, contentType string, expire time.Duration) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3/R2 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, cfg.Region)

	if expire <= 0 {
		expire = 15 * time.Minute
	}
	if expire > 24*time.Hour {
		expire = 24 * time.Hour
	}
	expiresSec := int(expire.Seconds())

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid S3 endpoint URL: %w", err)
	}
	host := u.Host

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", cfg.AccessKey, credentialScope))
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(expiresSec))
	q.Set("X-Amz-SignedHeaders", "host")

	canonicalQuery := buildCanonicalQueryFromValues(q)
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	canonicalURI := fmt.Sprintf("/%s/%s", cfg.Bucket, cleanKey)
	canonicalReq := fmt.Sprintf("PUT\n%s\n%s\n%s\nhost\nUNSIGNED-PAYLOAD", canonicalURI, canonicalQuery, canonicalHeaders)
	canonicalHash := sha256Hex([]byte(canonicalReq))

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, canonicalHash)

	kDate := hmacSHA256([]byte("AWS4"+cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	return fmt.Sprintf("%s/%s/%s?%s&X-Amz-Signature=%s", cfg.Endpoint, cfg.Bucket, cleanKey, canonicalQuery, signature), nil
}

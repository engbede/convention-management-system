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

func buildCanonicalQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	parts := strings.Split(rawQuery, "&")
	type qParam struct {
		k, v string
	}
	var params []qParam
	for _, p := range parts {
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		k := url.QueryEscape(kv[0])
		v := ""
		if len(kv) > 1 {
			v = url.QueryEscape(kv[1])
		}
		params = append(params, qParam{k: k, v: v})
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].k < params[j].k
	})
	var builder strings.Builder
	for i, param := range params {
		if i > 0 {
			builder.WriteString("&")
		}
		builder.WriteString(param.k)
		builder.WriteString("=")
		builder.WriteString(param.v)
	}
	return builder.String()
}

func signS3Request(req *http.Request, cfg *S3Config, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	host := req.URL.Host
	var canonicalHeaders strings.Builder
	var signedHeaders []string

	if ct := req.Header.Get("Content-Type"); ct != "" {
		canonicalHeaders.WriteString(fmt.Sprintf("content-type:%s\n", ct))
		signedHeaders = append(signedHeaders, "content-type")
	}

	canonicalHeaders.WriteString(fmt.Sprintf("host:%s\n", host))
	signedHeaders = append(signedHeaders, "host")

	if acl := req.Header.Get("x-amz-acl"); acl != "" {
		canonicalHeaders.WriteString(fmt.Sprintf("x-amz-acl:%s\n", acl))
		signedHeaders = append(signedHeaders, "x-amz-acl")
	}

	canonicalHeaders.WriteString(fmt.Sprintf("x-amz-content-sha256:%s\n", payloadHash))
	signedHeaders = append(signedHeaders, "x-amz-content-sha256")

	canonicalHeaders.WriteString(fmt.Sprintf("x-amz-date:%s\n", amzDate))
	signedHeaders = append(signedHeaders, "x-amz-date")

	signedHeadersStr := strings.Join(signedHeaders, ";")
	canonicalHeadersStr := canonicalHeaders.String()

	canonicalQuery := buildCanonicalQuery(req.URL.RawQuery)
	canonicalReq := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", req.Method, req.URL.Path, canonicalQuery, canonicalHeadersStr, signedHeadersStr, payloadHash)
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

	req, err := http.NewRequest(http.MethodPut, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-acl", "public-read")

	payloadHash := sha256Hex(bodyBytes)
	signS3Request(req, cfg, payloadHash)

	client := &http.Client{Timeout: 120 * time.Second}
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

// InitiateS3MultipartUpload begins an S3-compatible multipart upload and returns the upload ID.
func InitiateS3MultipartUpload(key, contentType string) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3 object storage is not configured")
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
	req.Header.Set("x-amz-acl", "public-read")

	signS3Request(req, cfg, sha256Hex([]byte("")))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("initiate multipart upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
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

// UploadS3Part uploads a single chunk/part to an ongoing S3 multipart upload.
func UploadS3Part(key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	reqURL := fmt.Sprintf("%s/%s/%s?partNumber=%d&uploadId=%s", cfg.Endpoint, cfg.Bucket, cleanKey, partNumber, url.QueryEscape(uploadID))

	partBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read part data: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, reqURL, bytes.NewReader(partBytes))
	if err != nil {
		return "", err
	}

	payloadHash := sha256Hex(partBytes)
	signS3Request(req, cfg, payloadHash)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload part %d failed: %w", partNumber, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload part %d returned status %d: %s", partNumber, resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	if etag == "" {
		etag = payloadHash
	}

	return etag, nil
}

// CompleteS3MultipartUpload finalizes an S3 multipart upload after all parts are uploaded.
func CompleteS3MultipartUpload(key, uploadID string, parts []S3CompletedPart) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3 object storage is not configured")
	}

	// Sort parts by PartNumber ascending (required by S3 API)
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
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("complete multipart upload returned status %d: %s", resp.StatusCode, string(body))
	}

	if cfg.PublicURL != "" {
		return fmt.Sprintf("%s/%s", cfg.PublicURL, cleanKey), nil
	}
	return fmt.Sprintf("%s/%s/%s", cfg.Endpoint, cfg.Bucket, cleanKey), nil
}

// AbortS3MultipartUpload cancels an active S3 multipart upload and cleans up stored parts.
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

// GeneratePresignedGetURL creates a presigned GET URL with SigV4 query parameters.
func GeneratePresignedGetURL(key string, expire time.Duration) (string, error) {
	cfg := GetS3Config()
	if cfg == nil {
		return "", errors.New("S3 object storage is not configured")
	}

	cleanKey := strings.TrimPrefix(key, "/")
	if cfg.PublicURL != "" {
		return fmt.Sprintf("%s/%s", cfg.PublicURL, cleanKey), nil
	}

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, cfg.Region)

	if expire <= 0 {
		expire = 24 * time.Hour
	}
	expiresSec := int(expire.Seconds())

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return "", err
	}
	host := u.Host

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", cfg.AccessKey, credentialScope))
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(expiresSec))
	q.Set("X-Amz-SignedHeaders", "host")

	canonicalQuery := buildCanonicalQuery(q.Encode())
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


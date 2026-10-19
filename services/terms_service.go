package services

import (
	"errors"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
)

// DefaultTermsVersion is the baseline active version for platform terms of use.
const DefaultTermsVersion = "v1.0"

// GetActiveTermsVersion retrieves the current mandatory terms version from platform settings.
func GetActiveTermsVersion() string {
	val := repository.GetSetting("active_terms_version")
	trimmed := strings.TrimSpace(val)
	if trimmed != "" {
		return trimmed
	}
	return DefaultTermsVersion
}

// SetActiveTermsVersion updates the active terms version requiring re-acceptance by members.
func SetActiveTermsVersion(version string) error {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return errors.New("terms version cannot be empty")
	}
	return repository.SaveSetting("active_terms_version", trimmed)
}

// CheckTermsAcceptance verifies whether a given user has accepted the active terms version.
func CheckTermsAcceptance(userID int) (bool, string, *models.TermsAcceptance, error) {
	requiredVersion := GetActiveTermsVersion()
	accepted, err := repository.HasUserAcceptedTerms(userID, requiredVersion)
	if err != nil {
		return false, requiredVersion, nil, err
	}

	latest, _ := repository.GetLatestTermsAcceptance(userID)
	return accepted, requiredVersion, latest, nil
}

// AcceptTerms records a user's explicit consent to the platform terms of use.
func AcceptTerms(userID int, version, ip, userAgent string) error {
	requiredVersion := GetActiveTermsVersion()

	// If no version specified by client, assume agreeing to the active version
	ver := strings.TrimSpace(version)
	if ver == "" {
		ver = requiredVersion
	}

	// Clean IP address
	cleanIP := strings.TrimSpace(ip)
	if idx := strings.Index(cleanIP, ":"); idx != -1 && !strings.Contains(cleanIP, "]") {
		// remove port if standard host:port format
		cleanIP = cleanIP[:idx]
	}

	return repository.RecordTermsAcceptance(userID, ver, cleanIP, strings.TrimSpace(userAgent))
}


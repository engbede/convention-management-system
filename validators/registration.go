package validators

import (
	"errors"
	"regexp"
	"strings"
)

func ValidatePhone(phone string) error {

	phone = strings.TrimSpace(phone)

	if phone == "" {
		return errors.New("phone number is required")
	}

	if len(phone) != 11 {
		return errors.New("phone number must be exactly 11 digits")
	}

	match, _ := regexp.MatchString(`^[0-9]+$`, phone)
	if !match {
		return errors.New("phone number must contain only digits")
	}

	if phone[0] != '0' {
		return errors.New("phone number must start with 0")
	}

	return nil
}

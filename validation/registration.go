package validation

import (
	"errors"
	"regexp"
	"strings"

	"convention-management-system/models"
)

func ValidateRegistration(r models.Registration) map[string]string {

	errorsMap := make(map[string]string)

	if strings.TrimSpace(r.FullName) == "" {
		errorsMap["fullname"] = "Full name is required."
	}

	if r.Gender != "Male" && r.Gender != "Female" {
		errorsMap["gender"] = "Please select gender."
	}

	if r.Age <= 0 {
		errorsMap["age"] = "Enter a valid age."
	}

	if strings.TrimSpace(r.Phone) == "" {

		errorsMap["phone"] = "Phone number is required."

	} else {

		matched, _ := regexp.MatchString(`^\d{11}$`, r.Phone)

		if !matched {

			errorsMap["phone"] =
				"Phone number must contain exactly 11 digits."

		}
	}

	if strings.TrimSpace(r.Circuit) == "" {
		errorsMap["circuit"] = "Circuit is required."
	}

	if strings.TrimSpace(r.LocalChurch) == "" {
		errorsMap["local_church"] = "Local church is required."
	}

	if strings.TrimSpace(r.Membership) == "" {
		errorsMap["membership"] = "Membership is required."
	}

	if r.BibleStudyGroup == 0 {
		errorsMap["bible_study_group"] =
			"Select a Bible Study Group."
	}

	if strings.TrimSpace(r.EmergencyContactName) == "" {
		errorsMap["emergency_contact_name"] =
			"Emergency contact name is required."
	}

	if strings.TrimSpace(r.EmergencyContactPhone) == "" {

		errorsMap["emergency_contact_phone"] =
			"Emergency contact phone is required."

	} else {

		matched, _ := regexp.MatchString(`^\d{11}$`,
			r.EmergencyContactPhone)

		if !matched {

			errorsMap["emergency_contact_phone"] =
				"Emergency phone must contain exactly 11 digits."

		}
	}

	if strings.TrimSpace(r.ArrivalDate) == "" {
		errorsMap["arrival_date"] = "Arrival date is required."
	}

	return errorsMap
}

func HasErrors(errs map[string]string) error {

	if len(errs) > 0 {

		return errors.New("validation failed")

	}

	return nil
}

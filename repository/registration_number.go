package repository

import (
	"fmt"
)

func GenerateRegistrationNumber(id int) string {
	return fmt.Sprintf("ADYF-2026-%06d", id)
}

package helpers

import "fmt"

// RegistrationNumber returns a formatted registration number.
func RegistrationNumber(id int) string {
	return fmt.Sprintf("MCN-APA-2026-%04d", id)
}

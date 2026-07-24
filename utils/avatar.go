package utils

import "strings"

func Initials(fullName string) string {

	parts := strings.Fields(fullName)

	if len(parts) == 0 {
		return "U"
	}

	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:1])
	}

	first := parts[0][:1]
	last := parts[len(parts)-1][:1]

	return strings.ToUpper(first + last)
}

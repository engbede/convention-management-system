package helpers

import (
	"fmt"
	"os"
)

func Placeholder(n int) string {
	if os.Getenv("DATABASE_URL") != "" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

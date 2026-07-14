package models

type FormData struct {
	Title      string
	Action     string
	ButtonText string

	Registration Registration
	Convention   Convention

	Errors map[string]string

	Notices []Notice

	Success string

	Error string
}

package models

type Convention struct {
	ID        int
	Name      string
	Theme     string
	Venue     string
	StartDate string
	EndDate   string
	Year      int
	Active    bool
}

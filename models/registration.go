package models

type Registration struct {
	ID                 int
	ConventionID       int
	RegistrationNumber string
	QRCode             string

	FullName string
	Gender   string
	Age      int

	Phone string
	Email string

	Circuit     string
	LocalChurch string

	Membership string
	Position   string

	MaritalStatus string
	Occupation    string

	EmergencyContactName  string
	EmergencyContactPhone string

	Relationship string
	Address      string

	ArrivalDate string

	FirstTimeAttendee bool

	CheckedIn bool

	BibleStudyGroup int
}

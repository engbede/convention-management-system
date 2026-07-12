package models

type DashboardStats struct {
	TotalRegistrations int

	MaleCount   int
	FemaleCount int

	MemberCount    int
	NonMemberCount int

	FirstTimeCount int

	MarriedCount   int
	SingleCount    int
	DivorcedCount  int
	CheckedIn      int
	Pending        int
	AttendanceRate float64

	Circuits []CircuitStat
}

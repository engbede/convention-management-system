package models

type Finance struct {
	ID int

	Type string // Income or Expense

	Category string

	Description string

	Amount float64

	RecordedBy string

	Date string
}
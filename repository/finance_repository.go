package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)
func CreateFinance(finance models.Finance) error {

	_, err := database.DB.Exec(`
		INSERT INTO finance (
			type,
			category,
			description,
			amount,
			recorded_by,
			date
		)
		VALUES ($1,$2,$3,$4,$5,$6)
	`,
		finance.Type,
		finance.Category,
		finance.Description,
		finance.Amount,
		finance.RecordedBy,
		finance.Date,
	)

	return err
}

func GetAllFinance() ([]models.Finance, error) {

	rows, err := database.DB.Query(`
		SELECT
			id,
			type,
			category,
			description,
			amount,
			recorded_by,
			date
		FROM finance
		ORDER BY date DESC, id DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var finances []models.Finance

	for rows.Next() {

		var finance models.Finance

		err := rows.Scan(
			&finance.ID,
			&finance.Type,
			&finance.Category,
			&finance.Description,
			&finance.Amount,
			&finance.RecordedBy,
			&finance.Date,
		)

		if err != nil {
			return nil, err
		}

		finances = append(
			finances,
			finance,
		)
	}

	return finances, nil
}

func GetFinanceByID(id int) (models.Finance, error) {

	var finance models.Finance

	err := database.DB.QueryRow(`
		SELECT
			id,
			type,
			category,
			description,
			amount,
			recorded_by,
			date
		FROM finance
		WHERE id=$1
	`, id).Scan(
		&finance.ID,
		&finance.Type,
		&finance.Category,
		&finance.Description,
		&finance.Amount,
		&finance.RecordedBy,
		&finance.Date,
	)

	return finance, err
}

func UpdateFinance(finance models.Finance) error {

	_, err := database.DB.Exec(`
		UPDATE finance
		SET
			type=$1,
			category=$2,
			description=$3,
			amount=$4,
			recorded_by=$5,
			date=$6
		WHERE id=$7
	`,
		finance.Type,
		finance.Category,
		finance.Description,
		finance.Amount,
		finance.RecordedBy,
		finance.Date,
		finance.ID,
	)

	return err
}

func DeleteFinance(id int) error {

	_, err := database.DB.Exec(`
		DELETE FROM finance
		WHERE id=$1
	`, id)

	return err
}

func GetFinanceSummary() (
	totalIncome float64,
	totalExpense float64,
	balance float64,
	totalTransactions int,
	err error,
) {

	err = database.DB.QueryRow(`
		SELECT
			COALESCE(SUM(amount),0)
		FROM finance
		WHERE type='Income'
	`).Scan(&totalIncome)

	if err != nil {
		return
	}

	err = database.DB.QueryRow(`
		SELECT
			COALESCE(SUM(amount),0)
		FROM finance
		WHERE type='Expense'
	`).Scan(&totalExpense)

	if err != nil {
		return
	}

	err = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM finance
	`).Scan(&totalTransactions)

	if err != nil {
		return
	}

	balance = totalIncome - totalExpense

	return
}
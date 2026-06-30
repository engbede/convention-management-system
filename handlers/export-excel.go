package handlers

import (
	"net/http"

	"convention-management-system/repository"

	"github.com/xuri/excelize/v2"
)

func ExportExcel(w http.ResponseWriter, r *http.Request) {

	registrations, err := repository.GetAllRegistrations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()

	sheet := "Registrations"

	f.SetSheetName("Sheet1", sheet)

	// Headers
	headers := []string{
		"ID", "Full Name", "Gender", "Age", "Phone",
		"Circuit", "Local Church", "Membership",
		"Position", "Marital Status", "Occupation",
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Data rows
	for i, reg := range registrations {
		row := i + 2

		f.SetCellValue(sheet, getCell(1, row), reg.ID)
		f.SetCellValue(sheet, getCell(2, row), reg.FullName)
		f.SetCellValue(sheet, getCell(3, row), reg.Gender)
		f.SetCellValue(sheet, getCell(4, row), reg.Age)
		f.SetCellValue(sheet, getCell(5, row), reg.Phone)
		f.SetCellValue(sheet, getCell(6, row), reg.Circuit)
		f.SetCellValue(sheet, getCell(7, row), reg.LocalChurch)
		f.SetCellValue(sheet, getCell(8, row), reg.Membership)
		f.SetCellValue(sheet, getCell(9, row), reg.Position)
		f.SetCellValue(sheet, getCell(10, row), reg.MaritalStatus)
		f.SetCellValue(sheet, getCell(11, row), reg.Occupation)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=registrations.xlsx")

	_ = f.Write(w)
}

func getCell(col, row int) string {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	return cell
}

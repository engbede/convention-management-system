package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/repository"

	"github.com/go-pdf/fpdf"
)

func ExportPDF(w http.ResponseWriter, r *http.Request) {

	registrations, err := repository.GetAllRegistrations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pdf := fpdf.New("L", "mm", "A4", "")

	pdf.SetTitle("Convention Registration Report", false)

	pdf.AddPage()

	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(0, 10, "Convention Management System")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 14)
	pdf.Cell(0, 10, "Youth Convention Registration Report")
	pdf.Ln(15)

	pdf.SetFont("Arial", "B", 10)

	headers := []string{
		"ID",
		"Name",
		"Gender",
		"Age",
		"Phone",
		"Circuit",
		"Church",
		"Membership",
	}

	widths := []float64{
		15,
		45,
		20,
		15,
		35,
		40,
		55,
		30,
	}

	for i, header := range headers {

		pdf.CellFormat(
			widths[i],
			8,
			header,
			"1",
			0,
			"C",
			false,
			0,
			"",
		)
	}

	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)

	for _, reg := range registrations {

		pdf.CellFormat(15, 8, strconv.Itoa(reg.ID), "1", 0, "C", false, 0, "")
		pdf.CellFormat(45, 8, reg.FullName, "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 8, reg.Gender, "1", 0, "C", false, 0, "")
		pdf.CellFormat(15, 8, strconv.Itoa(reg.Age), "1", 0, "C", false, 0, "")
		pdf.CellFormat(35, 8, reg.Phone, "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 8, reg.Circuit, "1", 0, "", false, 0, "")
		pdf.CellFormat(55, 8, reg.LocalChurch, "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 8, reg.Membership, "1", 0, "", false, 0, "")

		pdf.Ln(-1)
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set(
		"Content-Disposition",
		`attachment; filename="registrations.pdf"`,
	)

	if err := pdf.Output(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
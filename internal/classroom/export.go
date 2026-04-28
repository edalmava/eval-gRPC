package classroom

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sia/pkg/models"
	"sort"

	"github.com/xuri/excelize/v2"
)

// ExportQuestionToCSV genera un buffer con los datos de una pregunta en formato CSV.
func ExportQuestionToCSV(q *models.ActiveQuestion) ([]byte, error) {
	if q == nil {
		return nil, fmt.Errorf("pregunta nula")
	}

	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)

	// Cabecera
	header := []string{"Pregunta", "Tipo", "Estudiante", "ID Cliente", "Respuesta", "Es Correcta", "Fecha/Hora"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, ans := range q.Answers {
		isCorrect := "N/A"
		if q.Type == "MULTIPLE_CHOICE" || q.Type == "1" { // "1" es el valor enum en JSON a veces
			if ans.Answer == q.CorrectOption {
				isCorrect = "SÍ"
			} else {
				isCorrect = "NO"
			}
		}

		row := []string{
			q.Text,
			q.Type,
			ans.StudentName,
			ans.ClientID,
			ans.Answer,
			isCorrect,
			ans.Timestamp.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// ExportSessionToExcel genera un archivo Excel con el resumen de la sesión y detalle por pregunta.
func ExportSessionToExcel(history map[string]*models.ActiveQuestion) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// 1. Hoja de Resumen de Estudiantes
	sheetResumen := "Resumen General"
	f.SetSheetName("Sheet1", sheetResumen)

	// Cabeceras Resumen
	headers := []string{"Estudiante", "ID Cliente", "Preguntas Respondidas", "Aciertos", "Efectividad"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetResumen, cell, h)
	}

	// Estilo para cabeceras
	styleHeader, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DCE6F1"}, Pattern: 1},
	})
	f.SetRowStyle(sheetResumen, 1, 1, styleHeader)

	// Calcular estadísticas por estudiante
	type Stats struct {
		Name      string
		Total     int
		Correctas int
	}
	studentStats := make(map[string]*Stats)

	// Ordenar preguntas por fecha
	var qIDs []string
	for id := range history {
		qIDs = append(qIDs, id)
	}
	sort.Slice(qIDs, func(i, j int) bool {
		return history[qIDs[i]].CreatedAt.Before(history[qIDs[j]].CreatedAt)
	})

	for _, qID := range qIDs {
		q := history[qID]
		for cid, ans := range q.Answers {
			if _, ok := studentStats[cid]; !ok {
				studentStats[cid] = &Stats{Name: ans.StudentName}
			}
			studentStats[cid].Total++
			if (q.Type == "MULTIPLE_CHOICE" || q.Type == "1") && ans.Answer == q.CorrectOption {
				studentStats[cid].Correctas++
			}
		}
	}

	// Escribir datos de resumen
	rowIdx := 2
	for cid, s := range studentStats {
		efectividad := 0.0
		if s.Total > 0 {
			efectividad = float64(s.Correctas) / float64(s.Total)
		}
		f.SetCellValue(sheetResumen, fmt.Sprintf("A%d", rowIdx), s.Name)
		f.SetCellValue(sheetResumen, fmt.Sprintf("B%d", rowIdx), cid)
		f.SetCellValue(sheetResumen, fmt.Sprintf("C%d", rowIdx), s.Total)
		f.SetCellValue(sheetResumen, fmt.Sprintf("D%d", rowIdx), s.Correctas)
		f.SetCellDefault(sheetResumen, fmt.Sprintf("E%d", rowIdx), fmt.Sprintf("%.1f%%", efectividad*100))
		rowIdx++
	}

	// 2. Una hoja por cada pregunta
	for i, qID := range qIDs {
		q := history[qID]
		sheetName := fmt.Sprintf("P%d", i+1)
		// Limitar nombre de hoja a 31 caracteres (límite de Excel)
		if len(q.Text) > 20 {
			shortText := q.Text
			if len(shortText) > 20 {
				shortText = shortText[:20]
			}
			sheetName = fmt.Sprintf("P%d-%s", i+1, shortText)
		}
		f.NewSheet(sheetName)

		// Info de la pregunta
		f.SetCellValue(sheetName, "A1", "PREGUNTA:")
		f.SetCellValue(sheetName, "B1", q.Text)
		f.SetCellValue(sheetName, "A2", "CORRECTA:")
		f.SetCellValue(sheetName, "B2", q.CorrectOption)
		f.SetRowStyle(sheetName, 1, 2, styleHeader)

		// Cabeceras de respuestas
		qHeaders := []string{"Estudiante", "Respuesta", "Resultado", "Fecha/Hora"}
		for j, h := range qHeaders {
			cell, _ := excelize.CoordinatesToCellName(j+1, 4)
			f.SetCellValue(sheetName, cell, h)
		}
		f.SetRowStyle(sheetName, 4, 4, styleHeader)

		ansIdx := 5
		for _, ans := range q.Answers {
			res := "---"
			if q.Type == "MULTIPLE_CHOICE" || q.Type == "1" {
				if ans.Answer == q.CorrectOption {
					res = "CORRECTO"
				} else {
					res = "INCORRECTO"
				}
			}
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", ansIdx), ans.StudentName)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", ansIdx), ans.Answer)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", ansIdx), res)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", ansIdx), ans.Timestamp.Format("15:04:05"))
			ansIdx++
		}
		f.SetColWidth(sheetName, "A", "D", 20)
	}

	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

package payments

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/reports"
	"fmt"
	"time"
)

func PaymentsUpdate(db *database.MongoClient, payments []models.Payment, user models.User) ([]models.MonthReport, error) {
	paymentsForClient := PaymentsForClientFunc(payments)
	//mes actual
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	//mes orevio
	lastMonth := timeNow.AddDate(0, -1, 0)
	yearAfter := lastMonth.Year()
	monthAfter := int(lastMonth.Month())
	var errors []error
	var updatesReports []models.MonthReport
	for _, client := range paymentsForClient {
		var defaultReport models.MonthReport
		previusReport, err := reports.FindOrCreateReport(db, user.Email, client.ClientUuid, yearAfter, monthAfter, defaultReport)
		if err != nil {
			fmt.Println("Error al buscar o crear reporte", err)
			errors = append(errors, err)
			continue
		}
		currentReport, err := reports.FindOrCreateReport(db, user.Email, client.ClientUuid, year, month, previusReport)
		if err != nil {
			fmt.Println("Error al buscar o crear reporte", err)
			errors = append(errors, err)
			continue
		}
		upReport, err := AddNewPayments(db, client.Payments, currentReport, previusReport)
		if err != nil {
			fmt.Println("Error alguardar los pagos", err)
			errors = append(errors, err)
			continue
		}
		updatesReports = append(updatesReports, upReport)
	}
	if len(errors) > 0 {
		return updatesReports, fmt.Errorf("error al guardar paymentes")
	}
	return updatesReports, nil
}
func PaymentsForClientFunc(payments []models.Payment) []models.PaymentsForClient {
	var arraysPayments []models.PaymentsForClient
	for _, payment := range payments {
		found := false
		for i, array := range arraysPayments {
			if payment.ClientUuid == array.ClientUuid {
				arraysPayments[i].Payments = append(array.Payments, payment)
				found = true
				break
			}
		}
		if !found {
			var client models.PaymentsForClient
			client.ClientUuid = payment.ClientUuid
			client.Payments = append(client.Payments, payment)
			arraysPayments = append(arraysPayments, client)
		}
	}
	return arraysPayments
}

func AddNewPayments(db *database.MongoClient, payments []models.Payment, currentReport models.MonthReport, previusReport models.MonthReport) (models.MonthReport, error) {
	//Verificamos que los pago no los registraramos previamente en el mes anteior y este, y por error de comunicasion front-back se estan
	//re-enviado, contemplamos el mes anterior por si el error sucede el 31 del mes y no se intenta reintegrar los datos.
	newPayments := CheckNewPayments(payments, previusReport)
	fmt.Println("Paym Last", newPayments)
	newPayments = CheckNewPayments(newPayments, currentReport)
	fmt.Println("Paym Curre", newPayments)

	currentReport.Payments = append(currentReport.Payments, newPayments...)

	//subimos los cambios al reporte
	_, err := db.UpdateReport(&currentReport)
	if err != nil {
		fmt.Println("Error al actualizar reporte", err)
		return currentReport, err
	}
	return currentReport, err
}

func CheckNewPayments(payments []models.Payment, report models.MonthReport) []models.Payment {
	var newPayments []models.Payment
	for _, payment := range payments {
		// Verificar si el ID del pago coincide con algún otro pago en report.payments
		found := false
		for _, reportPayment := range report.Payments {
			if payment.Uuid == reportPayment.Uuid {
				found = true
				break // Si ya encontramos una coincidencia, podemos salir del bucle interno
			}
		}
		if !found {
			// No se encontró coincidencia, agregar el pago a newPayments
			newPayments = append(newPayments, payment)
		}
	}
	return newPayments
}

func PaymentsForMonth(payments []models.Payment) []models.PaymentsForMonth {
	var arraysPayments []models.PaymentsForMonth
	for _, payment := range payments {
		found := false
		for i, array := range arraysPayments {
			if int(payment.Date.Month()) == array.Month && int(payment.Date.Year()) == array.Year {
				arraysPayments[i].Payments = append(array.Payments, payment)
				found = true
				break
			}
		}
		if !found {
			var dateSlice models.PaymentsForMonth
			dateSlice.Month = int(payment.Date.Month())
			dateSlice.Year = int(payment.Date.Year())
			dateSlice.Payments = append(dateSlice.Payments, payment)
			arraysPayments = append(arraysPayments, dateSlice)
		}
	}
	return arraysPayments
}

package payments

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/reports"
	"time"
)

func PaymentsUpdate(db *database.MongoClient, payments []models.Payment, user models.User) {
	paymentsForClient := PaymentsForClientFunc(payments)
	//mes actual
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	//mes orevio
	lastMonth := timeNow.AddDate(0, -1, 0)
	yearAfter := lastMonth.Year()
	monthAfter := int(lastMonth.Month())
	for _, client := range paymentsForClient {
		previusReport, err := reports.FindOrCreateReport(db, user.Email, client.ClientUuid)
		//buscar Report actual y anterior

	}
}
func PaymentsForClientFunc(payments []models.Payment) []models.PaymentsForClient {
	var arraysPayments []models.PaymentsForClient
	for _, payment := range payments {
		found := false
		for _, array := range arraysPayments {
			if payment.ClientUuid == array.ClientUuid {
				array.Payments = append(array.Payments, payment)
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

func AddNewPayments(db *database.MongoClient, payments []models.Payment, currentReport models.MonthReport, previusReport models.MonthReport) {
	//Verificamos que los pago no los registraramos previamente en algun mes anteior y este, y por error de comunicasion front-back se estan
	//re-enviado, contemplamos el mes anterior por si el error sucede el 31 del mes y no se intenta reintegrar los datos.
	newPayments := CheckNewPayments(payments, previusReport)
	newPayments = CheckNewPayments(newPayments, currentReport)

	currentReport.Payments = append(currentReport.Payments, newPayments...)

	//subimos los cambios al reporte
	db.UpdateReport(&currentReport)

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

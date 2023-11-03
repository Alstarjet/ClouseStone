package charges

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/reports"
	"fmt"
	"time"
)

func ChargesUpdate(db *database.MongoClient, charges []models.Charge, user models.User) ([]models.MonthReport, error) {
	chargesForClient := ChargesForClientFunc(charges)
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
	for _, client := range chargesForClient {
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
		upReport, err := AddNewCharges(db, client.Charges, currentReport, previusReport)
		if err != nil {
			fmt.Println("Error alguardar los pagos", err)
			errors = append(errors, err)
			continue
		}
		updatesReports = append(updatesReports, upReport)
	}
	if len(errors) > 0 {
		return updatesReports, fmt.Errorf("error al guardar chargees")
	}
	return updatesReports, nil
}
func ChargesForClientFunc(charges []models.Charge) []models.ChargesForClient {
	var arraysCharges []models.ChargesForClient
	for _, charge := range charges {
		found := false
		for i, array := range arraysCharges {
			if charge.ClientUuid == array.ClientUuid {
				arraysCharges[i].Charges = append(array.Charges, charge)
				found = true
				break
			}
		}
		if !found {
			var client models.ChargesForClient
			client.ClientUuid = charge.ClientUuid
			client.Charges = append(client.Charges, charge)
			arraysCharges = append(arraysCharges, client)
		}
	}
	return arraysCharges
}

func AddNewCharges(db *database.MongoClient, charges []models.Charge, currentReport models.MonthReport, previusReport models.MonthReport) (models.MonthReport, error) {
	//Verificamos que los pago no los registraramos previamente en el mes anteior y este, y por error de comunicasion front-back se estan
	//re-enviado, contemplamos el mes anterior por si el error sucede el 31 del mes y no se intenta reintegrar los datos.
	newCharges := CheckNewCharges(charges, previusReport)
	fmt.Println("Paym Last", newCharges)
	newCharges = CheckNewCharges(newCharges, currentReport)
	fmt.Println("Paym Curre", newCharges)

	currentReport.Charges = append(currentReport.Charges, newCharges...)

	//subimos los cambios al reporte
	_, err := db.UpdateReport(&currentReport)
	if err != nil {
		fmt.Println("Error al actualizar reporte", err)
		return currentReport, err
	}
	return currentReport, err
}

func CheckNewCharges(charges []models.Charge, report models.MonthReport) []models.Charge {
	var newCharges []models.Charge
	for _, charge := range charges {
		// Verificar si el ID del pago coincide con algún otro pago en report.charges
		found := false
		for _, reportCharge := range report.Charges {
			if charge.Uuid == reportCharge.Uuid {
				found = true
				break // Si ya encontramos una coincidencia, podemos salir del bucle interno
			}
		}
		if !found {
			// No se encontró coincidencia, agregar el pago a newCharges
			newCharges = append(newCharges, charge)
		}
	}
	return newCharges
}

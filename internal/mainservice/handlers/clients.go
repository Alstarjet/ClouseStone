package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/charges"
	"financial-Assistant/internal/mainservice/moduls/clients"
	"financial-Assistant/internal/mainservice/moduls/payments"
	"fmt"
	"io"
	"log"
	"net/http"
)

func UploadDataSchedule(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.RequestUpload
		fmt.Println("AA")
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(req, &newData)
		if err != nil {
			response, _ := json.Marshal(err)
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
			return
		}
		emailRequest := r.Context().Value("Email").(string)
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		clientsUpload := clients.ClientsUpload(db, newData.Clients, user)
		reportPayments, paymentsUpload := payments.PaymentsUpdate(db, newData.Payments, user)
		reporteCharges, chargesUpload := charges.ChargesUpdate(db, newData.Charges, user)
		var responseReporte []models.MonthReport
		for _, reportP := range reportPayments {
			found := false
			for _, reportC := range reporteCharges {
				if reportP.ClientUuid == reportC.ClientUuid {
					responseReporte = append(responseReporte, reportC)
					found = true
				}
			}
			if found {
				continue
			} else {
				responseReporte = append(responseReporte, reportP)
			}
		}
		for _, reportC := range reporteCharges {
			found := false
			for _, reportP := range reportPayments {
				if reportP.ClientUuid == reportC.ClientUuid {
					found = true
				}
			}
			if found {
				continue
			} else {
				responseReporte = append(responseReporte, reportC)
			}
		}

		if clientsUpload != nil || paymentsUpload != nil || chargesUpload != nil {
			response := models.BackupResponse{
				Message: "Intenta Respaldar más tarde",
			}
			jsonResponse, _ := json.Marshal(response)

			w.WriteHeader(http.StatusAccepted)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonResponse)
		} else {
			response := models.BackupResponse{
				Message: "Datos Respaldados con Éxito",
				Reports: responseReporte,
			}
			jsonResponse, _ := json.Marshal(response)

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonResponse)
		}

	})
}

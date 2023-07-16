package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

func AddPayment(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.Payment
		var report models.MonthReport
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		_ = json.Unmarshal(req, &newData)

		updateReq := r.Context().Value("Email").(string)
		user, err := db.FindUser(updateReq)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		timeNow := time.Now()
		year := timeNow.Year()
		month := int(timeNow.Month())
		report, err = db.FindReport(user.Email, newData.ClientUuid, year, month)
		if err == mongo.ErrNoDocuments {

			log.Println(err, year, month)
		} else {
			if err != nil {
				log.Println(err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
		respJSON, err := json.Marshal(report)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
	})
}
func MatchReport(db *database.MongoClient, user models.User, clientUuid string) (models.MonthReport, error, bool) {
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	//Buscamos Reporte del mes en curso
	report, err := db.FindReport(user.Email, clientUuid, year, month)
	if err == mongo.ErrNoDocuments {
		//Reporte no encontrado
		//Buscar reporte del mes anterior para sacar balance actual y crear el reporte del mes.

		return report, nil, false
	} else {
		if err != nil {
			log.Println(err)
			return report, err, true
		}
		return report, nil, true
	}
}
func AddPayments() {

}

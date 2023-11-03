package clients

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

func ClientsUpload(db *database.MongoClient, clients []models.ClientRegister, user models.User) error {
	var errors []error
	for _, client := range clients {
		upload := CreateNewCliente(db, client, user)
		if !upload {
			errors = append(errors, fmt.Errorf("error al guardar cliente: %v", client))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("error al guardar clientes")
	}
	return nil
}
func CreateNewCliente(db *database.MongoClient, newClient models.ClientRegister, user models.User) bool {
	_, err := db.FindClient(user.Email, newClient.ClientUuid, newClient.Name)
	if err == mongo.ErrNoDocuments {
		var newReport models.MonthReport
		newClient.UserEmail = user.Email
		timeNow := time.Now()
		year := timeNow.Year()
		month := int(timeNow.Month())
		_, err = db.AddClient(newClient)
		if err != nil {
			log.Println(err)
			return false
		}
		newReport.ClientUuid = newClient.ClientUuid
		newReport.LastDebt = 0
		newReport.Month = month
		newReport.Year = year
		newReport.UserEmail = user.Email
		newReport.Charges = []models.Charge{}
		newReport.Payments = []models.Payment{}
		_, err = db.AddReport(&newReport)
		if err != nil {
			log.Println(err)
			return false
		}
		return true
	} else {
		if err != nil {
			return false
		} else {
			return true
		}
	}

}

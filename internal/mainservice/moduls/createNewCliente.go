package scripts

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

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

package clients

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"

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
		newClient.UserEmail = user.Email
		_, err = db.AddClient(newClient)
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

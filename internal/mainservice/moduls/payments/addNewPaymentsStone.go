package payments

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
)

func PaymentsUpdateStone(db *database.MongoClient, payments []models.Payment, user models.User) error {
	//Comprobacion de documentos para insertar o hacer update
	return nil
}

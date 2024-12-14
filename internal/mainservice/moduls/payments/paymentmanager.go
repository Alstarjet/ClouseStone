package payments

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func PaymentsUploadStone(db *database.MongoClient, payments []models.Payment, user models.User) ([]string, error) {
	var OkIds []string
	for _, payment := range payments {
		filter := bson.D{
			{Key: "uuid", Value: payment.UUID},
			{Key: "usermongoid", Value: user.ID.Hex()},
		}
		paymentCloud, err := db.FindPayment(filter)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				payment.UserMongoID = user.ID.Hex()
				id, err := db.AddPayment(payment)
				if err != nil {
					return nil, err
				}
				if oid, ok := id.(primitive.ObjectID); ok {
					// Si lo es, podemos llamar a Hex() para obtener su representación como cadena
					idStr := oid.Hex()
					OkIds = append(OkIds, idStr)

				}
			} else {
				return OkIds, err
			}
		} else {
			payment.ID = paymentCloud.ID
			payment.UserMongoID = paymentCloud.UserMongoID

			err = db.UpdatePayment(filter, payment)
			if err != nil {
				return OkIds, err
			}

			OkIds = append(OkIds, payment.ID.Hex())
		}
	}
	return OkIds, nil
}

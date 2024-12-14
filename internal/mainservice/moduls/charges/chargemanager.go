package charges

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ChargesUploadStone(db *database.MongoClient, charges []models.Charge, user models.User) ([]string, error) {
	var OkIds []string
	for _, charge := range charges {
		filter := bson.D{
			{Key: "uuid", Value: charge.UUID},
			{Key: "usermongoid", Value: user.ID.Hex()},
		}
		chargeCloud, err := db.FindCharge(filter)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				charge.UserMongoID = user.ID.Hex()
				id, err := db.AddCharge(charge)
				if err != nil {
					return nil, err
				}
				if oid, ok := id.(primitive.ObjectID); ok {
					idStr := oid.Hex()
					OkIds = append(OkIds, idStr)
				}
			} else {
				return OkIds, err
			}
		} else {
			charge.ID = chargeCloud.ID
			charge.UserMongoID = chargeCloud.UserMongoID

			err = db.UpdateCharge(filter, charge)
			if err != nil {
				return OkIds, err
			}

			OkIds = append(OkIds, charge.ID.Hex())
		}
	}
	return OkIds, nil
}

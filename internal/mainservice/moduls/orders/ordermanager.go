package orders

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func OrdersUploadStone(db *database.MongoClient, orders []models.Charge, user models.User) ([]string, error) {
	var OkIds []string
	for _, order := range orders {
		filter := bson.D{
			{Key: "uuid", Value: order.UUID},
			{Key: "usermongoid", Value: user.ID.Hex()},
		}
		orderCloud, err := db.FindOrder(filter)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				order.UserMongoID = user.ID.Hex()
				id, err := db.AddOrder(order)
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
			order.ID = orderCloud.ID
			order.UserMongoID = orderCloud.UserMongoID

			err = db.UpdateOrder(filter, order)
			if err != nil {
				return OkIds, err
			}

			OkIds = append(OkIds, order.ID.Hex())
		}
	}
	return OkIds, nil
}

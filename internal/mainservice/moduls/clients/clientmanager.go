package clients

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ClientsUploadStone(db *database.MongoClient, clients []models.Client, user models.User) ([]string, error) {
	var OkIds []string
	for _, client := range clients {
		// El servidor sella la hora de llegada (createat/updateat son del cliente).
		client.BackupAt = time.Now().UTC()
		filter := bson.D{
			{Key: "uuid", Value: client.UUID},
			{Key: "usermongoid", Value: user.ID.Hex()},
		}
		clientCloud, err := db.FindClient(filter)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				client.UserMongoID = user.ID.Hex()
				id, err := db.AddClient(client)
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
			client.ID = clientCloud.ID
			client.UserMongoID = clientCloud.UserMongoID

			err = db.UpdateClient(filter, client)
			if err != nil {
				return OkIds, err
			}

			OkIds = append(OkIds, client.ID.Hex())
		}
	}
	return OkIds, nil
}

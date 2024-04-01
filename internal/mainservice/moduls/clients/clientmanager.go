package clients

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ClientsUploadStone(db *database.MongoClient, clients []models.Client, user models.User) ([]string, error) {
	var OkIds []string
	for _, client := range clients {
		filter := bson.D{
			{Key: "clientuuid", Value: client.ClientUUID},
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
				fmt.Println("Se Agrego nuevo cliente Cliente:")
				fmt.Println(id)
				if oid, ok := id.(primitive.ObjectID); ok {
					// Si lo es, podemos llamar a Hex() para obtener su representación como cadena
					idStr := oid.Hex()
					OkIds = append(OkIds, idStr)

				}
			}
		} else {
			client.ID = clientCloud.ID
			client.UserMongoID = clientCloud.UserMongoID
			clientMap := structToBSONMap(client)
			update := bson.M{"$set": clientMap}
			err = db.UpdateClient(filter, update)
			if err != nil {
				return OkIds, err
			}
			fmt.Println("Actualizamos Cliente:")
			fmt.Println(client)
			OkIds = append(OkIds, client.ID.Hex())
		}
	}
	return OkIds, nil
}
func structToBSONMap(input interface{}) bson.M {
	bsonBytes, err := bson.Marshal(input)
	if err != nil {
		log.Fatal(err)
	}

	var resultMap bson.M
	err = bson.Unmarshal(bsonBytes, &resultMap)
	if err != nil {
		log.Fatal(err)
	}

	return resultMap
}

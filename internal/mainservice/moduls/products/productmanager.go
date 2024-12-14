package products

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ProductsUploadStone(db *database.MongoClient, products []models.Product, user models.User) ([]string, error) {
	var OkIds []string
	for _, product := range products {
		filter := bson.D{
			{Key: "uuid", Value: product.UUID},
			{Key: "usermongoid", Value: user.ID.Hex()},
		}
		productCloud, err := db.FindProduct(filter)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				product.UserMongoID = user.ID.Hex()
				id, err := db.AddProduct(product)
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
			product.ID = productCloud.ID
			product.UserMongoID = productCloud.UserMongoID

			err = db.UpdateProduct(filter, product)
			if err != nil {
				return OkIds, err
			}

			OkIds = append(OkIds, product.ID.Hex())
		}
	}
	return OkIds, nil
}

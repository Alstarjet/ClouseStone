package database

import (
	"context"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ExpirePaidPlans pasa a la versión gratis (typeclient = Quartz) las cuentas de pago cuya vigencia
// (planexpiresat) ya pasó, y regresa cuántas cambió. Las de pago sin fecha no se tocan, y
// planexpiresat se conserva para que se vea cuándo venció.
//
// La escritura repite el filtro de la fecha: si un pago le alargó la vigencia a una cuenta entre
// la lectura y la escritura, ésa ya no cambia. Correrla dos veces no hace daño.
func (mc *MongoClient) ExpirePaidPlans(ctx context.Context, now time.Time) (int64, error) {
	collection := mc.client.Database(DataBase).Collection("users")
	filter := bson.M{"typeclient": plans.Paid, "planexpiresat": bson.M{"$lt": now}}

	cursor, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return 0, fmt.Errorf("buscar planes vencidos: %w", err)
	}
	var found []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &found); err != nil {
		return 0, fmt.Errorf("leer planes vencidos: %w", err)
	}
	if len(found) == 0 {
		log.Printf("ExpirePaidPlans: ninguna cuenta vencida")
		return 0, nil
	}

	ids := make([]primitive.ObjectID, len(found))
	hex := make([]string, len(found))
	for i, doc := range found {
		ids[i], hex[i] = doc.ID, doc.ID.Hex()
	}
	filter["_id"] = bson.M{"$in": ids}
	result, err := collection.UpdateMany(ctx, filter, bson.M{"$set": bson.M{"typeclient": plans.Free}})
	if err != nil {
		return 0, fmt.Errorf("pasar a la versión gratis: %w", err)
	}
	log.Printf("ExpirePaidPlans: %d de %d cuentas vencidas pasaron a la versión gratis: %s",
		result.ModifiedCount, len(found), strings.Join(hex, ", "))
	return result.ModifiedCount, nil
}

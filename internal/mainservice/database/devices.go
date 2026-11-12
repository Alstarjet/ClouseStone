package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const devices = "devices"

// En el documento de dispositivos de cada usuario viven los refresh tokens de
// todos sus dispositivos. Por eso aquí sólo hay escrituras atómicas sobre un
// dispositivo ($set de sus campos, $push, $pull): nunca un $set del documento
// completo, que regresaría el token viejo de un dispositivo que lo renovó
// mientras tanto y cerraría su sesión (I-07).

// AddDevice crea el documento de dispositivos del usuario si todavía no existe
// (upsert con $setOnInsert). Si ya existe no toca nada, y dos logins a la vez no
// chocan.
func (mc *MongoClient) AddDevice(ctx context.Context, doc models.UserDevices) error {
	update := bson.D{{Key: "$setOnInsert", Value: bson.D{
		{Key: "usermongoid", Value: doc.UserMongoID},
		{Key: "username", Value: doc.UserName},
		{Key: "useremail", Value: doc.UserEmail},
		// Siempre una lista (nunca null): si no, el $push de AddNewDevice falla
		{Key: "devices", Value: bson.A{}},
	}}}
	collection := mc.client.Database(DataBase).Collection(devices)
	_, err := collection.UpdateOne(ctx, bson.D{{Key: "_id", Value: doc.ID}}, update, options.Update().SetUpsert(true))
	return err
}

func (mc *MongoClient) FindDevice(ctx context.Context, filter interface{}) (models.UserDevices, error) {
	collection := mc.client.Database(DataBase).Collection(devices)
	var result models.UserDevices
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return models.UserDevices{}, err
	}
	return result, nil
}

// UpdateDeviceRefreshToken cambia el refresh token del dispositivo (de todas sus
// entradas, si estuviera repetido). found es false si el usuario no tiene ese
// dispositivo.
func (mc *MongoClient) UpdateDeviceRefreshToken(ctx context.Context, userID primitive.ObjectID, deviceUUID string, newToken string, newExpiry time.Time) (found bool, err error) {
	filter := bson.D{{Key: "_id", Value: userID}, {Key: "devices.uuid", Value: deviceUUID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "devices.$[elem].refreshtoken.token", Value: newToken},
			{Key: "devices.$[elem].refreshtoken.dateend", Value: newExpiry},
		}},
	}
	// Los filtros de array identifican el dispositivo dentro del documento
	updateOptions := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{bson.M{"elem.uuid": deviceUUID}},
	})

	collection := mc.client.Database(DataBase).Collection(devices)
	result, err := collection.UpdateOne(ctx, filter, update, updateOptions)
	if err != nil {
		log.Println("Error updating refresh token:", err)
		return false, err
	}
	return result.MatchedCount > 0, nil
}

// AddNewDevice agrega el dispositivo al usuario sólo si todavía no lo tiene, en
// una sola escritura: dos logins a la vez del mismo dispositivo no lo repiten.
// pushed es false si ya estaba (o si el usuario no tiene documento de
// dispositivos).
func (mc *MongoClient) AddNewDevice(ctx context.Context, userID primitive.ObjectID, newDevice models.Device) (pushed bool, err error) {
	filter := bson.D{
		{Key: "_id", Value: userID},
		{Key: "devices.uuid", Value: bson.D{{Key: "$ne", Value: newDevice.UUID}}},
	}
	update := bson.D{
		{Key: "$push", Value: bson.M{
			"devices": newDevice,
		}},
	}

	collection := mc.client.Database(DataBase).Collection(devices)
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Println("Error adding new device:", err)
		return false, err
	}
	return result.MatchedCount > 0, nil
}

func (mc *MongoClient) RemoveDeviceByUUID(ctx context.Context, userID primitive.ObjectID, deviceUUID string) error {
	// Filtrar por el ID del usuario
	filter := bson.D{{Key: "_id", Value: userID}}

	// Definir la actualización para eliminar el dispositivo por UUID
	removeUpdate := bson.D{
		{Key: "$pull", Value: bson.M{
			"devices": bson.M{"uuid": deviceUUID},
		}},
	}

	collection := mc.client.Database(DataBase).Collection(devices)

	_, err := collection.UpdateOne(ctx, filter, removeUpdate)
	if err != nil {
		log.Println("Error removing device by UUID:", err)
		return err
	}

	return nil
}

// ClearDeviceIDs vacía la cola de pendientes de un dispositivo (legado: la lee
// /GetData y la vacía /DeleteIds). Sólo toca las cuatro listas de ese
// dispositivo, nunca su refresh token ni los de los demás.
func (mc *MongoClient) ClearDeviceIDs(ctx context.Context, userID primitive.ObjectID, deviceUUID string) error {
	update, arrayFilters := clearDeviceIDsUpdate(deviceUUID)
	collection := mc.client.Database(DataBase).Collection(devices)
	_, err := collection.UpdateOne(ctx, bson.D{{Key: "_id", Value: userID}}, update,
		options.Update().SetArrayFilters(arrayFilters))
	return err
}

// clearDeviceIDsUpdate es la escritura de ClearDeviceIDs: listas vacías en el
// dispositivo deviceUUID ($[d]) y nada más.
func clearDeviceIDsUpdate(deviceUUID string) (bson.D, options.ArrayFilters) {
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "devices.$[d].clientids", Value: bson.A{}},
		{Key: "devices.$[d].chargeids", Value: bson.A{}},
		{Key: "devices.$[d].orderids", Value: bson.A{}},
		{Key: "devices.$[d].paymentids", Value: bson.A{}},
	}}}
	return update, options.ArrayFilters{Filters: []interface{}{bson.M{"d.uuid": deviceUUID}}}
}

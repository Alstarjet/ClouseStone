package handlers

import (
	"context"
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetData baja lo que quedó en la cola de pendientes (legado) del dispositivo.
// La app de producción todavía lo llama desde una pantalla vieja (y si
// /GetDataSince respondiera 404). Ya es inofensivo: la cola no crece (nadie la
// escribe desde nov-2026, I-07) y sólo regresa documentos actuales del usuario,
// nunca uno vacío. Se quita cuando se despliegue el frontend sin esa pantalla
// (I-14).
func GetData(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		deviceid := queryParams.Get("deviceid")

		emailRequest, ok := r.Context().Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		user, err := db.FindUser(ctx, emailRequest)
		if err != nil {
			log.Printf("GetData: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		DataDevice, err := devices.ConsultIDs(ctx, db, user, deviceid)
		if err != nil {
			log.Printf("GetData: consult IDs error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		DataResponse, err := ConsutDocumentsForDevice(ctx, db, DataDevice, user.ID.Hex())
		if err != nil {
			log.Printf("GetData: consult documents error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(DataResponse); err != nil {
			log.Printf("GetData: encode response error: %v", err)
		}
	})
}

// ConsutDocumentsForDevice resuelve la cola del dispositivo a los documentos
// actuales, con una consulta por colección y sólo del usuario. Un id que ya no
// existe se omite (antes se agregaba un documento vacío, sin uuid, que la app
// guardaba) y un error de la base es error (antes se ignoraba).
func ConsutDocumentsForDevice(ctx context.Context, db accountReader, device models.Device, usermongoid string) (models.AllData, error) {
	var data models.AllData
	var err error
	if filter, ok := queueFilter(usermongoid, device.ClientIDs); ok {
		if data.Clients, err = db.FindAllClients(ctx, filter); err != nil {
			return models.AllData{}, err
		}
	}
	if filter, ok := queueFilter(usermongoid, device.ChargeIDs); ok {
		if data.Charges, err = db.FindAllCharges(ctx, filter); err != nil {
			return models.AllData{}, err
		}
	}
	if filter, ok := queueFilter(usermongoid, device.OrderIDs); ok {
		if data.Orders, err = db.FindAllOrders(ctx, filter); err != nil {
			return models.AllData{}, err
		}
	}
	if filter, ok := queueFilter(usermongoid, device.PaymentIDs); ok {
		if data.Payments, err = db.FindAllPayments(ctx, filter); err != nil {
			return models.AllData{}, err
		}
	}
	return data, nil
}

// queueFilter arma la consulta de los ids de una cola: sólo documentos del
// usuario, cada uno una vez aunque su id esté repetido. Los ids que no son de
// Mongo se ignoran; sin ninguno válido no hay nada que consultar.
func queueFilter(usermongoid string, ids []string) (bson.D, bool) {
	oids := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		if oid, err := primitive.ObjectIDFromHex(id); err == nil {
			oids = append(oids, oid)
		}
	}
	if len(oids) == 0 {
		return nil, false
	}
	return bson.D{
		{Key: "usermongoid", Value: usermongoid},
		{Key: "_id", Value: bson.D{{Key: "$in", Value: oids}}},
	}, true
}

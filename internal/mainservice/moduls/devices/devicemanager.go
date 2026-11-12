package devices

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
)

// La cola de pendientes por dispositivo (chargeids, clientids, orderids,
// paymentids) es del modelo anterior de sincronización. Desde nov-2026 ya nadie
// la escribe: /UploadData dejó de llenarla porque crecía sin límite y su
// reescritura completa pisaba los refresh tokens (I-07). Sólo queda lo que ya
// tenía, que /GetData todavía puede bajar y /DeleteIds vaciar (la app de
// producción los llama desde una pantalla vieja; se quitan cuando se despliegue
// el frontend sin ella).

// ConsultIDs regresa la cola de pendientes del dispositivo (vacía si el
// dispositivo no está registrado).
func ConsultIDs(ctx context.Context, db *database.MongoClient, user models.User, deviceuuid string) (models.Device, error) {
	var newDevice models.Device
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	deviceDoc, err := db.FindDevice(ctx, filter)
	if err != nil {
		return newDevice, err
	}
	for _, device := range deviceDoc.Devices {
		if device.UUID == deviceuuid {
			return device, nil
		}
	}
	return newDevice, nil
}

// DeleteIDsForDevice vacía la cola de pendientes del dispositivo con una sola
// escritura sobre sus listas: no toca su refresh token ni los de los demás.
func DeleteIDsForDevice(ctx context.Context, db *database.MongoClient, user models.User, deviceuuid string) error {
	return db.ClearDeviceIDs(ctx, user.ID, deviceuuid)
}

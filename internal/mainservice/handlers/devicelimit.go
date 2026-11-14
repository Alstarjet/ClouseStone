package handlers

import (
	"context"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Tope de dispositivos (plans.DeviceLimit): una cuenta puede tener la sesión abierta
// en ese número de dispositivos a la vez. Sólo cuentan las sesiones vigentes (el
// refresh token no ha vencido), así que la lista se libera sola. El dispositivo que
// ya está en la lista siempre puede volver a entrar.
//
// Para cerrar la sesión de un dispositivo a pedido de la persona, se quita su entrada
// del documento de dispositivos (la consola gluzie-admin lo hace): su siguiente
// /GetJwt responde 401 y la app pide volver a entrar.
//
// No es atómico a propósito: dos logins a la vez desde dos dispositivos nuevos
// podrían dejar uno de más. Es raro y no vale un candado en la base.

// deviceFinder lee el documento de dispositivos. Lo cumple *database.MongoClient.
type deviceFinder interface {
	FindDevice(ctx context.Context, filter interface{}) (models.UserDevices, error)
}

// otherActiveSessions cuenta los dispositivos de la lista, sin contar deviceUUID,
// cuya sesión sigue vigente en now.
func otherActiveSessions(devices []models.Device, deviceUUID string, now time.Time) int {
	n := 0
	for _, device := range devices {
		if device.UUID != deviceUUID && device.Refreshtoken.DateEnd.After(now) {
			n++
		}
	}
	return n
}

// deviceLimitReached dice si deviceUUID ya no cabe: la cuenta tiene plans.DeviceLimit
// sesiones vigentes en otros dispositivos. Sin documento de dispositivos (primer
// login) siempre cabe.
func deviceLimitReached(ctx context.Context, db deviceFinder, user models.User, deviceUUID string, now time.Time) (bool, error) {
	doc, err := db.FindDevice(ctx, bson.D{{Key: "_id", Value: user.ID}})
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return otherActiveSessions(doc.Devices, deviceUUID, now) >= plans.DeviceLimit, nil
}

// writeDeviceLimit es la respuesta del login cuando el dispositivo no cabe.
func writeDeviceLimit(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"error":   "device_limit",
		"limit":   plans.DeviceLimit,
		"message": fmt.Sprintf("Tu cuenta ya está abierta en %d dispositivos. Cierra sesión en alguno o escríbenos para cerrarla.", plans.DeviceLimit),
	})
}

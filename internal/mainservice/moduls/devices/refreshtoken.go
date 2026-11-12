package devices

import (
	"context"
	"errors"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"time"
)

// AddDeviceAndRefreshToken registra el dispositivo del login con su refresh
// token (sólo el hash). Si el dispositivo ya estaba, sólo reemplaza su token.
//
// Todo son escrituras atómicas sobre el documento de dispositivos, sin leerlo ni
// reescribirlo completo (ahí viven los tokens de los demás dispositivos, I-07):
// primero se asegura el documento, luego se cambia el token del dispositivo y,
// si no lo tenía, se agrega sin repetirlo aunque entren dos logins a la vez.
func AddDeviceAndRefreshToken(ctx context.Context, db *database.MongoClient, user models.User, refreshToken string, expires time.Time, deviceUUID string) error {
	hashedToken := utilities.HashToken(refreshToken)

	err := db.AddDevice(ctx, models.UserDevices{
		ID:          user.ID,
		UserMongoID: user.ID.Hex(),
		UserName:    user.Name + " " + user.LastName,
		UserEmail:   user.Email,
	})
	if err != nil {
		return err
	}

	// Dos vueltas: si entre el cambio de token y el $push otro login del mismo
	// dispositivo lo agregó, en la segunda ya existe y sólo se cambia su token.
	for attempt := 0; attempt < 2; attempt++ {
		found, err := db.UpdateDeviceRefreshToken(ctx, user.ID, deviceUUID, hashedToken, expires)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		pushed, err := db.AddNewDevice(ctx, user.ID, models.Device{
			UUID:       deviceUUID,
			ChargeIDs:  []string{},
			PaymentIDs: []string{},
			ClientIDs:  []string{},
			OrderIDs:   []string{},
			Refreshtoken: models.Refreshtoken{
				Token:   hashedToken,
				DateEnd: expires,
			},
		})
		if err != nil {
			return err
		}
		if pushed {
			return nil
		}
	}
	return errors.New("AddDeviceAndRefreshToken: no se pudo registrar el dispositivo")
}

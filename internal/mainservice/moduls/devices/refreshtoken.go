package devices

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"time"
)

func AddDeviceAndRefreshToken(db *database.MongoClient, Device models.UserDevices, RefreshToken string, Expires time.Time, UUID string) error {
	found := false
	for _, device := range Device.Devices {
		if device.UUID == UUID {
			err := db.UpdateDeviceRefreshToken(Device.ID, UUID, RefreshToken, Expires)
			if err != nil {
				return err
			}
			found = true

		}
	}
	if !found {
		newDevice := models.Device{
			UUID:       UUID,
			ChargeIDs:  []string{},
			PaymentIDs: []string{},
			ClientIDs:  []string{},
			OrderIDs:   []string{},
			Refreshtoken: models.Refreshtoken{
				Token:   RefreshToken,
				DateEnd: Expires,
			},
		}
		err := db.AddNewDevice(Device.ID, newDevice)
		if err != nil {
			return err
		}
	}
	return nil
}

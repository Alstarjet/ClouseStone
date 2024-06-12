package devices

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func DevicesUploadStone(db *database.MongoClient, clients []string, charges []string, orders []string, payments []string, user models.User, deviceuuid string) error {
	filter := bson.D{
		{Key: "_id", Value: user.ID.Hex()},
	}
	deviceDoc, err := db.FindDevice(filter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			var newDevicesDoc models.UserDevices
			var newDevice models.Device
			newDevice.UUID = deviceuuid
			newDevicesDoc.UserMongoID = user.ID.Hex()
			newDevicesDoc.Devices = append(newDevicesDoc.Devices, newDevice)
			_, err := db.AddDevice(newDevicesDoc)
			if err != nil {
				return err
			}
			deviceDoc = newDevicesDoc
		} else {
			return err
		}
	}
	for i := range deviceDoc.Devices {
		device := &deviceDoc.Devices[i] // Obtenemos una referencia al dispositivo en deviceDoc.Devices
		if device.UUID != deviceuuid {
			device.ClientIDs = append(device.ClientIDs, clients...)
			device.ChargeIDs = append(device.ChargeIDs, charges...)
			device.PaymentIDs = append(device.PaymentIDs, payments...)
			device.OrderIDs = append(device.OrderIDs, orders...)
		}
	}

	err = db.UpdateDevice(filter, deviceDoc)
	if err != nil {
		return err
	}
	return nil
}

func GetDevice(db *database.MongoClient, user models.User, deviceuuid string) (models.UserDevices, error) {
	//respondemos false si el dispositivo no esta registrado
	filter := bson.D{
		{Key: "_id", Value: user.ID},
	}
	var DeviceDoc models.UserDevices

	DeviceDoc, err := db.FindDevice(filter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			DeviceDoc.ID = user.ID
			DeviceDoc.UserMongoID = user.ID.Hex()
			DeviceDoc.UserName = user.Name + " " + user.LastName
			DeviceDoc.UserEmail = user.Email
			DeviceDoc.Devices = []models.Device{}
			_, err := db.AddDevice(DeviceDoc)
			if err != nil {
				return DeviceDoc, err
			}
			return DeviceDoc, nil
		} else {
			return DeviceDoc, err
		}
	}
	return DeviceDoc, nil
}
func ConsultIDs(db *database.MongoClient, user models.User, deviceuuid string) (models.Device, error) {
	var newDevice models.Device
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	deviceDoc, err := db.FindDevice(filter)
	if err != nil {
		return newDevice, err
	}
	for _, device := range deviceDoc.Devices {
		if device.UUID == deviceuuid {
			return device, nil
		}
	}
	return newDevice, err
}
func DeleteIDsForDevice(db *database.MongoClient, user models.User, deviceuuid string) error {
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	deviceDoc, err := db.FindDevice(filter)
	if err != nil {
		return err
	}
	for i := range deviceDoc.Devices {
		device := &deviceDoc.Devices[i] // Obtenemos una referencia al dispositivo en deviceDoc.Devices
		if device.UUID == deviceuuid {
			device.ClientIDs = nil
			device.ChargeIDs = nil
			device.PaymentIDs = nil
			device.OrderIDs = nil
		}
	}
	err = db.UpdateDevice(filter, deviceDoc)
	if err != nil {
		return err
	}
	return nil
}

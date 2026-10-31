package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID           primitive.ObjectID `json:"-" bson:"_id"`
	UUID         string             `json:"uuid"`
	Name         string             `json:"name"`
	Lastname     string             `json:"lastname"`
	Age          int                `json:"age"`
	City         string             `json:"city"`
	Neighborhood string             `json:"neighborhood"`
	Address      string             `json:"address"`
	Phone        interface{}        `json:"phone"`
	Daywork      string             `json:"daywork"`
	UserMongoID  string             `bson:"usermongoid" json:"-"`
	CreateAt     time.Time          `json:"createat"`
	UpdateAt     time.Time          `json:"updateat"`
	// BackupAt lo sella el SERVIDOR al recibir el documento (createat/updateat
	// vienen del reloj del dispositivo). Es la base del delta de /GetDataSince.
	BackupAt time.Time `json:"backupat" bson:"backupat"`
	Status   string    `json:"status"`
	// Type clasifica al cliente; por ahora sólo "blacklist" (lista negra).
	// Opcional: vacío = cliente normal. Con omitempty en bson, un respaldo que
	// no trae el campo (una versión de la app que no lo conoce) no lo borra.
	Type string `json:"type,omitempty" bson:"type,omitempty"`
	// ChangedFields son los campos (nombres JSON) que el dispositivo cambió
	// desde su último respaldo. Si el cliente ya existe, sólo se actualizan
	// ésos (ver clients.ClientsUploadStone). Viaja, pero no se guarda.
	ChangedFields []string `json:"changedfields,omitempty" bson:"-"`
}

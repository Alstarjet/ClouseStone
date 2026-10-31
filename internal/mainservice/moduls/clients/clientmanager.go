package clients

import (
	"errors"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// EditableFields son los campos de un cliente que un dispositivo puede mandar
// en changedfields (nombres JSON, iguales a los de Mongo). uuid, fechas y dueño
// no se cambian por esta vía.
var EditableFields = map[string]bool{
	"name": true, "lastname": true, "age": true, "city": true, "neighborhood": true,
	"address": true, "phone": true, "daywork": true, "status": true, "type": true,
}

// ErrUnknownField indica que un changedfields trae un campo que no es editable.
var ErrUnknownField = errors.New("changedfields: campo no editable")

// ValidateChangedFields revisa los changedfields de todos los clientes antes de
// guardar cualquiera, para rechazar el respaldo completo sin escribir nada.
func ValidateChangedFields(clients []models.Client) error {
	for _, client := range clients {
		for _, field := range client.ChangedFields {
			if !EditableFields[field] {
				return fmt.Errorf("%w: %q en el cliente %q", ErrUnknownField, field, client.UUID)
			}
		}
	}
	return nil
}

// changedFieldsUpdate arma la actualización de un cliente que ya existe con
// sólo los campos que cambiaron en el dispositivo, más updateat (del
// dispositivo) y backupat (del servidor). Así no pisa lo que otro dispositivo
// cambió en otros campos. Un campo que llega vacío y se guarda con omitempty
// (type) se quita del documento.
func changedFieldsUpdate(client models.Client) (bson.M, error) {
	data, err := bson.Marshal(client)
	if err != nil {
		return nil, fmt.Errorf("changedFieldsUpdate: %w", err)
	}
	var full bson.M
	if err := bson.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("changedFieldsUpdate: %w", err)
	}

	set := bson.M{"updateat": client.UpdateAt, "backupat": client.BackupAt}
	unset := bson.M{}
	for _, field := range client.ChangedFields {
		if value, ok := full[field]; ok {
			set[field] = value
		} else {
			unset[field] = ""
		}
	}
	update := bson.M{"$set": set}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	return update, nil
}

func ClientsUploadStone(db *database.MongoClient, clients []models.Client, user models.User) ([]string, error) {
	var OkIds []string
	for _, client := range clients {
		// El servidor sella la hora de llegada (createat/updateat son del cliente).
		client.BackupAt = time.Now().UTC()
		filter := bson.D{
			{Key: "uuid", Value: client.UUID},
			{Key: "usermongoid", Value: user.ID.Hex()},
		}
		clientCloud, err := db.FindClient(filter)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Nuevo en la nube: se guarda completo aunque traiga changedfields
				client.UserMongoID = user.ID.Hex()
				id, err := db.AddClient(client)
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
			client.ID = clientCloud.ID
			client.UserMongoID = clientCloud.UserMongoID

			if len(client.ChangedFields) > 0 {
				err = patchClient(db, filter, client)
			} else {
				// Sin changedfields (versiones anteriores de la app): el documento completo
				err = db.UpdateClient(filter, client)
			}
			if err != nil {
				return OkIds, err
			}

			OkIds = append(OkIds, client.ID.Hex())
		}
	}
	return OkIds, nil
}

func patchClient(db *database.MongoClient, filter bson.D, client models.Client) error {
	update, err := changedFieldsUpdate(client)
	if err != nil {
		return err
	}
	return db.PatchClient(filter, update)
}

package clients

import (
	"errors"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
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

// ErrPhoneType indica un teléfono que no es texto ni número (un objeto o una
// lista no se pueden guardar como teléfono).
var ErrPhoneType = errors.New("phone: se esperaba texto o número")

// Validate revisa lo que hace que un cliente nunca se pueda aceptar tal como
// viene: un campo no editable en changedfields o un teléfono que no es texto ni
// número. El respaldo rechaza sólo a ese cliente y guarda los demás (I-02).
func Validate(client models.Client) error {
	for _, field := range client.ChangedFields {
		if !EditableFields[field] {
			return fmt.Errorf("%w: %q", ErrUnknownField, field)
		}
	}
	switch client.Phone.(type) {
	case nil, string, float64:
		return nil
	default:
		return ErrPhoneType
	}
}

// Upsert arma el respaldo de un cliente en un solo upsert atómico (I-03):
//
//   - Sin changedfields (versiones anteriores de la app): el documento completo
//     encima del de la nube.
//   - Con changedfields: si el cliente ya está en la nube, sólo esos campos y
//     updateat (del dispositivo), para no pisar lo que otro dispositivo cambió en
//     otros campos; si es nuevo, se guarda completo (lo demás va en
//     $setOnInsert). Un campo cambiado que llega vacío y se guarda con omitempty
//     (type) se quita del documento.
//
// backupat, el dueño y el uuid los pone database.UpsertBackup.
func Upsert(client models.Client) (database.Upsert, error) {
	full, err := database.DeviceFields(client)
	if err != nil {
		return database.Upsert{}, fmt.Errorf("clients.Upsert: %w", err)
	}
	if len(client.ChangedFields) == 0 {
		return database.Upsert{UUID: client.UUID, Set: full}, nil
	}

	set := bson.M{"updateat": client.UpdateAt}
	unset := bson.M{}
	for _, field := range client.ChangedFields {
		if value, ok := full[field]; ok {
			set[field] = value
		} else {
			unset[field] = ""
		}
	}
	onInsert := bson.M{}
	for field, value := range full {
		if _, changed := set[field]; !changed {
			onInsert[field] = value
		}
	}
	return database.Upsert{UUID: client.UUID, Set: set, SetOnInsert: onInsert, Unset: unset}, nil
}

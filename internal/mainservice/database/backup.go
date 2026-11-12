package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Respaldo de /UploadData: todos los documentos de una colección se escriben en
// un solo viaje a Mongo (BulkWrite), cada uno con un upsert por (usermongoid,
// uuid). Antes eran dos viajes por documento, buscar y luego insertar o
// actualizar (I-03), y dos subidas a la vez del mismo documento podían crear dos
// copias (I-08).

// Upsert es el respaldo de un documento: se busca por (usermongoid, uuid) y, si
// no existe, se crea.
//
//   - Set se escribe siempre.
//   - SetOnInsert sólo cuando el documento es nuevo (así un cliente con
//     changedfields se guarda completo la primera vez y después sólo se tocan
//     los campos que cambiaron).
//   - Unset quita campos.
//
// Un campo va en uno solo de los tres; si se repite, gana Set y luego
// SetOnInsert (Mongo rechazaría la escritura entera).
type Upsert struct {
	UUID        string
	Set         bson.M
	SetOnInsert bson.M
	Unset       bson.M
}

// serverFields son los campos que el respaldo nunca toma del dispositivo: _id lo
// pone Mongo; usermongoid y uuid salen del filtro del upsert (en un documento
// nuevo, Mongo los copia de ahí); backupat es la hora de llegada que sella el
// servidor.
var serverFields = map[string]bool{"_id": true, "usermongoid": true, "uuid": true, "backupat": true}

// backupCollections son las colecciones que escribe el respaldo.
var backupCollections = map[string]bool{clients: true, charges: true, orders: true, payments: true}

// DeviceFields convierte un documento del respaldo (models.Client, Charge o
// Payment) en sus campos tal como se guardan en Mongo, sin los que pone el
// servidor (serverFields).
func DeviceFields(doc any) (bson.M, error) {
	data, err := bson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("DeviceFields: %w", err)
	}
	var fields bson.M
	if err := bson.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("DeviceFields: %w", err)
	}
	for field := range serverFields {
		delete(fields, field)
	}
	return fields, nil
}

// FullUpsert respalda el documento completo encima del que haya en la nube
// (cargos, pedidos, pagos y clientes sin changedfields).
func FullUpsert(uuid string, doc any) (Upsert, error) {
	fields, err := DeviceFields(doc)
	if err != nil {
		return Upsert{}, err
	}
	return Upsert{UUID: uuid, Set: fields}, nil
}

// UpsertBackup guarda en un solo viaje a Mongo los documentos de un respaldo en
// la colección (clients, charges, orders o payments) del usuario. Sella backupat
// con backupAt (la hora del servidor) en cada uno.
//
// Se puede repetir sin duplicar nada: el filtro es sólo la igualdad (usermongoid,
// uuid), las mismas llaves del índice único usermongoid_uuid_unique, y con ese
// índice Mongo reintenta él solo el upsert que choca porque otra subida acaba de
// crear el mismo documento. Si falla, regresa el error y el dispositivo reintenta
// el lote completo.
func (mc *MongoClient) UpsertBackup(ctx context.Context, collection, usermongoid string, backupAt time.Time, ups []Upsert) error {
	if !backupCollections[collection] {
		return fmt.Errorf("UpsertBackup: %q no es una colección del respaldo", collection)
	}
	if usermongoid == "" {
		return errors.New("UpsertBackup: falta el usuario")
	}
	if len(ups) == 0 {
		return nil
	}
	writes, err := upsertWrites(usermongoid, backupAt, ups)
	if err != nil {
		return err
	}
	// Ordenado (el predeterminado): si un documento llega dos veces en el mismo
	// lote, queda el último, como en el arreglo que mandó la app.
	if _, err := mc.client.Database(DataBase).Collection(collection).BulkWrite(ctx, writes); err != nil {
		return fmt.Errorf("UpsertBackup: %d documentos en %q: %w", len(ups), collection, err)
	}
	return nil
}

// Update es la actualización que recibe Mongo para este documento: backupat con
// la hora de llegada (backupAt) en $set, sin los campos del servidor y sin
// campos repetidos entre $set, $setOnInsert y $unset, para que Mongo nunca
// rechace la escritura por cómo se armó.
func (up Upsert) Update(backupAt time.Time) bson.M {
	set := bson.M{"backupat": backupAt}
	copyFields(set, up.Set)
	onInsert := bson.M{}
	copyFields(onInsert, up.SetOnInsert, set)
	unset := bson.M{}
	copyFields(unset, up.Unset, set, onInsert)

	update := bson.M{"$set": set}
	if len(onInsert) > 0 {
		update["$setOnInsert"] = onInsert
	}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	return update
}

// upsertWrites arma los upserts del BulkWrite, en el orden en que llegaron.
func upsertWrites(usermongoid string, backupAt time.Time, ups []Upsert) ([]mongo.WriteModel, error) {
	writes := make([]mongo.WriteModel, 0, len(ups))
	for _, up := range ups {
		if up.UUID == "" {
			return nil, errors.New("upsertWrites: un documento sin uuid")
		}
		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.D{{Key: "usermongoid", Value: usermongoid}, {Key: "uuid", Value: up.UUID}}).
			SetUpdate(up.Update(backupAt)).
			SetUpsert(true))
	}
	return writes, nil
}

// copyFields copia a dst los campos de src que no son del servidor ni están ya
// en alguno de taken.
func copyFields(dst, src bson.M, taken ...bson.M) {
	for field, value := range src {
		if serverFields[field] || inAny(field, taken) {
			continue
		}
		dst[field] = value
	}
}

func inAny(field string, docs []bson.M) bool {
	for _, doc := range docs {
		if _, ok := doc[field]; ok {
			return true
		}
	}
	return false
}

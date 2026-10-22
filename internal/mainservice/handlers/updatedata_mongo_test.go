package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/models"
	"net/http"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Estas pruebas necesitan MongoDB: se saltan si no está CLOUSESTONE_TEST_MONGO_URI
// (ver mongotest).

func registerUser(t *testing.T, db *database.MongoClient, email string) models.User {
	t.Helper()
	user := models.User{Name: "Ana", LastName: "López", Email: email}
	if _, err := db.RegisterUser(&user); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	return user
}

// storedDoc es lo que se revisa de cada documento guardado.
type storedDoc struct {
	id     primitive.ObjectID
	uuid   string
	detail any // nombre del cliente o monto del pago; nil en cargos y pedidos
}

// storedDocs lee lo que el usuario tiene guardado en cada colección del respaldo.
func storedDocs(t *testing.T, db *database.MongoClient, userID primitive.ObjectID) map[string][]storedDoc {
	t.Helper()
	filter := bson.D{{Key: "usermongoid", Value: userID.Hex()}}
	docs := map[string][]storedDoc{}

	clients, err := db.FindAllClients(filter)
	if err != nil {
		t.Fatalf("FindAllClients: %v", err)
	}
	for _, c := range clients {
		docs["clients"] = append(docs["clients"], storedDoc{c.ID, c.UUID, c.Name})
	}
	charges, err := db.FindAllCharges(filter)
	if err != nil {
		t.Fatalf("FindAllCharges: %v", err)
	}
	for _, c := range charges {
		docs["charges"] = append(docs["charges"], storedDoc{c.ID, c.UUID, nil})
	}
	orders, err := db.FindAllOrders(filter)
	if err != nil {
		t.Fatalf("FindAllOrders: %v", err)
	}
	for _, o := range orders {
		docs["orders"] = append(docs["orders"], storedDoc{o.ID, o.UUID, nil})
	}
	payments, err := db.FindAllPayments(filter)
	if err != nil {
		t.Fatalf("FindAllPayments: %v", err)
	}
	for _, p := range payments {
		docs["payments"] = append(docs["payments"], storedDoc{p.ID, p.UUID, p.Amount})
	}
	return docs
}

var backupUUIDs = map[string]string{"clients": "cli-1", "charges": "car-1", "orders": "ped-1", "payments": "pag-1"}

func TestUploadDataBackup(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerUser(t, db, testEmail)

	rec := serveUpload(t, db, testEmail, testDeviceID, validBackup)
	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d; se esperaba 200 (cuerpo: %s)", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body)
	}
	// ⚠️ El estado viaja con la clave "reports", no "status" (models.BackupResponse;
	// documentado en endpoints/data-uploaddata.md). La PWA sólo lee el código HTTP.
	want := map[string]any{"message": "Datos Respaldados con Éxito", "reports": float64(200), "typeclient": "Quartz"}
	for k, v := range want {
		if resp[k] != v {
			t.Errorf("respuesta[%q] = %v; se esperaba %v", k, resp[k], v)
		}
	}
	if len(resp) != len(want) {
		t.Errorf("respuesta = %v; se esperaban sólo las claves %v", resp, want)
	}

	docs := storedDocs(t, db, user.ID)
	for coll, uuid := range backupUUIDs {
		t.Run(coll, func(t *testing.T) {
			if got := docs[coll]; len(got) != 1 || got[0].uuid != uuid {
				t.Errorf("guardados = %+v; se esperaba sólo %q", got, uuid)
			}
		})
	}

	// Los tipos llegan intactos a Mongo.
	clients, err := db.FindAllClients(bson.D{{Key: "uuid", Value: "cli-1"}})
	if err != nil || len(clients) != 1 {
		t.Fatalf("cliente guardado = %v, %v", clients, err)
	}
	if c := clients[0]; c.Age != 40 || c.Phone != "2221234567" || c.UserMongoID != user.ID.Hex() || c.BackupAt.IsZero() {
		t.Errorf("cliente guardado = %+v", c)
	}
}

// Subir otra vez los mismos uuid actualiza los documentos en lugar de duplicarlos;
// el uuid sólo identifica dentro de cada usuario.
func TestUploadDataUpsertsByUUID(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerUser(t, db, testEmail)
	other := registerUser(t, db, "luis@ejemplo.mx")

	if rec := serveUpload(t, db, testEmail, testDeviceID, validBackup); rec.Code != http.StatusOK {
		t.Fatalf("primer respaldo: código = %d (%s)", rec.Code, rec.Body)
	}
	first := storedDocs(t, db, user.ID)

	edited := strings.NewReplacer(`"name":"Ana"`, `"name":"Ana María"`, `"amount":150`, `"amount":175.5`).Replace(validBackup)
	if rec := serveUpload(t, db, testEmail, testDeviceID, edited); rec.Code != http.StatusOK {
		t.Fatalf("segundo respaldo: código = %d (%s)", rec.Code, rec.Body)
	}
	if rec := serveUpload(t, db, "luis@ejemplo.mx", "dispositivo-luis", validBackup); rec.Code != http.StatusOK {
		t.Fatalf("respaldo de otro usuario: código = %d (%s)", rec.Code, rec.Body)
	}

	second := storedDocs(t, db, user.ID)
	wantDetail := map[string]any{"clients": "Ana María", "payments": 175.5}
	for coll, uuid := range backupUUIDs {
		t.Run(coll, func(t *testing.T) {
			got := second[coll]
			if len(got) != 1 || got[0].uuid != uuid {
				t.Fatalf("guardados = %+v; se esperaba un solo %q", got, uuid)
			}
			if got[0].id != first[coll][0].id {
				t.Errorf("_id cambió: %s → %s", first[coll][0].id.Hex(), got[0].id.Hex())
			}
			if want, ok := wantDetail[coll]; ok && got[0].detail != want {
				t.Errorf("no se actualizó: %v; se esperaba %v", got[0].detail, want)
			}
		})
	}

	// El mismo respaldo de otro usuario crea sus propios documentos.
	others := storedDocs(t, db, other.ID)
	for coll := range backupUUIDs {
		if got := others[coll]; len(got) != 1 || got[0].id == first[coll][0].id {
			t.Errorf("%s de otro usuario = %+v; se esperaba un documento propio", coll, got)
		}
	}
}

package devices

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"fmt"
	"slices"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// login registra el dispositivo como lo hace /login: documento de dispositivos
// del usuario (GetDevice) y su refresh token (AddDeviceAndRefreshToken).
func login(t *testing.T, db *database.MongoClient, user models.User, deviceUUID string) {
	t.Helper()
	doc, err := GetDevice(db, user, deviceUUID)
	if err != nil {
		t.Fatalf("GetDevice(%s): %v", deviceUUID, err)
	}
	if err := AddDeviceAndRefreshToken(db, doc, "token-"+deviceUUID, time.Now().Add(time.Hour), deviceUUID); err != nil {
		t.Fatalf("AddDeviceAndRefreshToken(%s): %v", deviceUUID, err)
	}
}

// ids devuelve los IDs que reciben los demás dispositivos tras n respaldos:
// el respaldo i sube <prefijo>-i.
func ids(prefix string, n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("%s-%d", prefix, i))
	}
	return out
}

// DevicesUploadStone anota en los DEMÁS dispositivos del usuario los IDs que se
// acaban de respaldar, para que los bajen con /GetData; el que sube no recibe nada.
// Necesita MongoDB (ver mongotest).
func TestDevicesUploadStone(t *testing.T) {
	db := mongotest.Connect(t)

	tests := []struct {
		name       string
		registered []string       // dispositivos que ya iniciaron sesión (nil = sin documento)
		uploader   string         // dispositivo que respalda
		uploads    int            // respaldos seguidos
		want       map[string]int // dispositivo → respaldos pendientes de bajar
	}{
		{"primer respaldo sin documento de dispositivos", nil, "A", 1, map[string]int{"A": 0}},
		{"el otro dispositivo recibe los IDs", []string{"A", "B"}, "A", 1, map[string]int{"A": 0, "B": 1}},
		{"respaldos seguidos se acumulan", []string{"A", "B", "C"}, "A", 2, map[string]int{"A": 0, "B": 2, "C": 2}},
		// Comportamiento actual: si el que sube no está registrado (p. ej. cerró sesión
		// con /CloseDevice y su access token sigue vigente) no se agrega, pero los
		// demás sí reciben sus IDs.
		{"dispositivo que sube sin estar registrado", []string{"B"}, "A", 1, map[string]int{"B": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Un usuario por caso: cada uno empieza sin documento de dispositivos.
			user := models.User{ID: primitive.NewObjectID(), Name: "Ana", Email: "ana@ejemplo.mx"}
			for _, uuid := range tt.registered {
				login(t, db, user, uuid)
			}
			for i := 1; i <= tt.uploads; i++ {
				err := DevicesUploadStone(db,
					[]string{fmt.Sprintf("cli-%d", i)}, []string{fmt.Sprintf("car-%d", i)},
					[]string{fmt.Sprintf("ped-%d", i)}, []string{fmt.Sprintf("pag-%d", i)},
					user, tt.uploader)
				if err != nil {
					t.Fatalf("DevicesUploadStone (respaldo %d): %v", i, err)
				}
			}

			doc, err := db.FindDevice(bson.D{{Key: "_id", Value: user.ID}})
			if err != nil {
				t.Fatalf("FindDevice: %v", err)
			}
			if doc.UserMongoID != user.ID.Hex() {
				t.Errorf("usermongoid = %q; se esperaba %q", doc.UserMongoID, user.ID.Hex())
			}
			if len(doc.Devices) != len(tt.want) {
				t.Errorf("dispositivos = %d; se esperaban %d", len(doc.Devices), len(tt.want))
			}
			for _, d := range doc.Devices {
				n, ok := tt.want[d.UUID]
				if !ok {
					t.Errorf("dispositivo inesperado %q", d.UUID)
					continue
				}
				pending := map[string][2][]string{
					"clientes": {d.ClientIDs, ids("cli", n)},
					"cargos":   {d.ChargeIDs, ids("car", n)},
					"pedidos":  {d.OrderIDs, ids("ped", n)},
					"pagos":    {d.PaymentIDs, ids("pag", n)},
				}
				for kind, p := range pending {
					if !slices.Equal(p[0], p[1]) {
						t.Errorf("%s: %s pendientes = %v; se esperaba %v", d.UUID, kind, p[0], p[1])
					}
				}
				// El respaldo no pisa el refresh token que guardó el login.
				if slices.Contains(tt.registered, d.UUID) && d.Refreshtoken.Token != utilities.HashToken("token-"+d.UUID) {
					t.Errorf("%s: se perdió el refresh token", d.UUID)
				}
			}
		})
	}
}

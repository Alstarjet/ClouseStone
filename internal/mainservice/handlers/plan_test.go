package handlers

import (
	"bytes"
	"encoding/json"
	"financial-Assistant/internal/mainservice/models"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// El login y /GetJwt regresan, junto al plan de siempre (typeclient), sus reglas
// y su vigencia. Un plan desconocido recibe las de la versión gratis.
func TestWithPlan(t *testing.T) {
	expires := time.Date(2027, 11, 10, 23, 59, 59, 0, time.UTC)
	tests := []struct {
		name        string
		user        models.User
		wantBackup  bool
		wantLimit   float64
		wantExpires any
	}{
		{"versión gratis, sin fecha", models.User{TypeClient: "Quartz"}, false, 50, nil},
		{"plan de pago con vigencia", models.User{TypeClient: "Amethyst", PlanExpiresAt: &expires}, true, 300, "2027-11-10T23:59:59Z"},
		{"plan de pago sin fecha", models.User{TypeClient: "Amethyst"}, true, 300, nil},
		{"plan desconocido", models.User{TypeClient: "Diamond"}, false, 50, nil},
		{"plan de pago ampliado a 500", models.User{TypeClient: "Amethyst", ClientLimit: 500}, true, 500, nil},
		{"versión gratis con tope propio: no cuenta", models.User{TypeClient: "Quartz", ClientLimit: 500}, false, 50, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := withPlan(models.JWTresponce{Toke: "token", UserName: "Ana López"}, tt.user)
			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if got["typeclient"] != tt.user.TypeClient || got["token"] != "token" || got["username"] != "Ana López" {
				t.Errorf("respuesta = %s", encoded)
			}
			if got["cloudbackup"] != tt.wantBackup || got["clientlimit"] != tt.wantLimit {
				t.Errorf("cloudbackup = %v, clientlimit = %v; se esperaba %v y %v", got["cloudbackup"], got["clientlimit"], tt.wantBackup, tt.wantLimit)
			}
			// Sin fecha viaja como null (la clave siempre está)
			value, ok := got["planexpiresat"]
			if !ok || value != tt.wantExpires {
				t.Errorf("planexpiresat = %v (¿está? %v); se esperaba %v", value, ok, tt.wantExpires)
			}
		})
	}
}

// Un clientlimit mal escrito a mano no cuenta (vale el tope del plan) y queda en el
// log con la cuenta, para corregirlo en la base.
func TestWithPlanIgnoresAnInvalidAccountLimit(t *testing.T) {
	id := primitive.NewObjectID()
	doc, err := bson.Marshal(bson.D{{Key: "_id", Value: id}, {Key: "typeclient", Value: "Amethyst"}, {Key: "clientlimit", Value: "500"}})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var user models.User
	if err := bson.Unmarshal(doc, &user); err != nil {
		t.Fatalf("la cuenta no se pudo leer: %v", err)
	}
	var out bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(previous) })

	response := withPlan(models.JWTresponce{}, user)
	if !response.CloudBackup || response.ClientLimit != 300 {
		t.Errorf("cloudbackup = %v, clientlimit = %d; se esperaba true y 300", response.CloudBackup, response.ClientLimit)
	}
	if logged := out.String(); !strings.Contains(logged, "clientlimit no válido") || !strings.Contains(logged, id.Hex()) {
		t.Errorf("el log no dice qué cuenta tiene el clientlimit mal: %q", logged)
	}
}

// Sólo un plan con respaldo en línea sigue; los demás reciben 403 plan_without_backup.
func TestAllowsCloudBackup(t *testing.T) {
	tests := []struct {
		typeclient string
		want       bool
	}{
		{"Amethyst", true},
		{"Quartz", false},
		{"", false},
		{"Diamond", false},
	}
	for _, tt := range tests {
		t.Run("plan "+tt.typeclient, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if got := allowsCloudBackup(rec, models.User{TypeClient: tt.typeclient}); got != tt.want {
				t.Fatalf("allowsCloudBackup = %v; se esperaba %v", got, tt.want)
			}
			if tt.want {
				if rec.Body.Len() != 0 {
					t.Errorf("con respaldo en línea no debe responder nada: %s", rec.Body)
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Errorf("código = %d; se esperaba 403", rec.Code)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"plan_without_backup"}` {
				t.Errorf("cuerpo = %s", got)
			}
		})
	}
}

// La vigencia la pone sólo quien administra la base: el registro no la toma del cliente.
func TestUserIgnoresClientPlanExpiry(t *testing.T) {
	body := `{"name":"Ana","planexpiresat":"2030-01-01T00:00:00Z"}`
	var user models.User
	if err := json.Unmarshal([]byte(body), &user); err != nil {
		t.Fatal(err)
	}
	if user.PlanExpiresAt != nil {
		t.Errorf("vigencia tomada del cliente: %v", user.PlanExpiresAt)
	}
}

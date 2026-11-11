package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		{"plan de pago con vigencia", models.User{TypeClient: "Amethyst", PlanExpiresAt: &expires}, true, 200, "2027-11-10T23:59:59Z"},
		{"plan de pago sin fecha", models.User{TypeClient: "Amethyst"}, true, 200, nil},
		{"plan desconocido", models.User{TypeClient: "Diamond"}, false, 50, nil},
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

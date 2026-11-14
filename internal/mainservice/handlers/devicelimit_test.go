package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var deviceNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

// session arma un dispositivo cuya sesión vence en days días (negativo: ya venció).
func session(uuid string, days int) models.Device {
	return models.Device{UUID: uuid, Refreshtoken: models.Refreshtoken{DateEnd: deviceNow.AddDate(0, 0, days)}}
}

// Sólo cuentan las sesiones vigentes de los otros dispositivos.
func TestOtherActiveSessions(t *testing.T) {
	devices := []models.Device{
		session("a", 20), session("b", 1), session("c", -1), session("d", 29), session("este", 10),
		{UUID: "venciendo-justo", Refreshtoken: models.Refreshtoken{DateEnd: deviceNow}},
	}
	if got := otherActiveSessions(devices, "este", deviceNow); got != 3 {
		t.Errorf("otherActiveSessions = %d; se esperaban 3 (a, b y d)", got)
	}
	if got := otherActiveSessions(devices, "nuevo", deviceNow); got != 4 {
		t.Errorf("otherActiveSessions con un dispositivo nuevo = %d; se esperaban 4", got)
	}
	if got := otherActiveSessions(nil, "nuevo", deviceNow); got != 0 {
		t.Errorf("sin dispositivos = %d", got)
	}
}

// fakeDevices es el documento de dispositivos en memoria.
type fakeDevices struct {
	doc models.UserDevices
	err error
}

func (f fakeDevices) FindDevice(_ context.Context, _ interface{}) (models.UserDevices, error) {
	return f.doc, f.err
}

func sessions(n int, days int) []models.Device {
	list := make([]models.Device, 0, n)
	for i := 0; i < n; i++ {
		list = append(list, session(string(rune('a'+i)), days))
	}
	return list
}

func TestDeviceLimitReached(t *testing.T) {
	user := models.User{ID: primitive.NewObjectID()}
	if plans.DeviceLimit != 5 {
		t.Fatalf("plans.DeviceLimit = %d; el usuario decidió 5", plans.DeviceLimit)
	}
	tests := []struct {
		name    string
		finder  fakeDevices
		device  string
		want    bool
		wantErr bool
	}{
		{"primer login: sin documento de dispositivos", fakeDevices{err: mongo.ErrNoDocuments}, "nuevo", false, false},
		{"4 sesiones vigentes: entra el quinto", fakeDevices{doc: models.UserDevices{Devices: sessions(4, 10)}}, "nuevo", false, false},
		{"5 sesiones vigentes: el sexto no entra", fakeDevices{doc: models.UserDevices{Devices: sessions(5, 10)}}, "nuevo", true, false},
		{"5 sesiones vigentes: uno de ellos vuelve a entrar", fakeDevices{doc: models.UserDevices{Devices: sessions(5, 10)}}, "a", false, false},
		{"5 sesiones vencidas no cuentan", fakeDevices{doc: models.UserDevices{Devices: sessions(5, -1)}}, "nuevo", false, false},
		{"la base falla", fakeDevices{err: errors.New("se cayó")}, "nuevo", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deviceLimitReached(context.Background(), tt.finder, user, tt.device, deviceNow)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v; ¿se esperaba error? %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("deviceLimitReached = %v; se esperaba %v", got, tt.want)
			}
		})
	}
}

// La respuesta del login cuando el dispositivo no cabe: 403 con el código, el tope
// y un mensaje para la persona.
func TestWriteDeviceLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	writeDeviceLimit(rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("código = %d; se esperaba 403", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("no es JSON: %s", rec.Body)
	}
	if body["error"] != "device_limit" || body["limit"] != float64(5) || body["message"] == "" {
		t.Errorf("cuerpo = %s", rec.Body)
	}
}

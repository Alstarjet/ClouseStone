package models

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// El tope propio de una cuenta se escribe a mano en la base: se lee como lo guarde
// cada herramienta, y un valor mal escrito no impide leer la cuenta (ni entrar).
func TestAccountLimitFromTheDatabase(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		wantValue   int
		wantInvalid bool
	}{
		{"entero de 32 bits (Compass, Atlas)", int32(500), 500, false},
		{"entero de 64 bits", int64(500), 500, false},
		{"decimal sin fracción", float64(500), 500, false},
		{"null: no tiene", nil, 0, false},
		{"con fracción", 500.5, 0, true},
		{"texto", "500", 0, true},
		{"cero", int32(0), 0, true},
		{"negativo", int32(-20), 0, true},
		{"verdadero", true, 0, true},
		{"enorme", float64(1e12), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := bson.Marshal(bson.D{
				{Key: "email", Value: "ana@ejemplo.mx"},
				{Key: "typeclient", Value: "Amethyst"},
				{Key: "clientlimit", Value: tt.value},
			})
			if err != nil {
				t.Fatalf("bson.Marshal: %v", err)
			}
			var user User
			if err := bson.Unmarshal(doc, &user); err != nil {
				t.Fatalf("la cuenta no se pudo leer: %v", err)
			}
			if got := user.ClientLimit.Value(); got != tt.wantValue {
				t.Errorf("Value() = %d; se esperaba %d", got, tt.wantValue)
			}
			if got := user.ClientLimit.Invalid(); got != tt.wantInvalid {
				t.Errorf("Invalid() = %v; se esperaba %v", got, tt.wantInvalid)
			}
			if user.Email != "ana@ejemplo.mx" || user.TypeClient != "Amethyst" {
				t.Errorf("el resto de la cuenta no se leyó: %+v", user)
			}
		})
	}
}

// Sin el campo, la cuenta no tiene tope propio, y no es un valor no válido.
func TestAccountLimitMissing(t *testing.T) {
	doc, err := bson.Marshal(bson.D{{Key: "email", Value: "ana@ejemplo.mx"}})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var user User
	if err := bson.Unmarshal(doc, &user); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}
	if user.ClientLimit.Value() != 0 || user.ClientLimit.Invalid() {
		t.Errorf("sin campo: Value() = %d, Invalid() = %v; se esperaba 0 y false", user.ClientLimit.Value(), user.ClientLimit.Invalid())
	}
}

// Una cuenta sin tope propio (como queda al registrarse) no escribe clientlimit, y
// una con tope lo guarda como número.
func TestAccountLimitToTheDatabase(t *testing.T) {
	doc, err := bson.Marshal(User{Email: "ana@ejemplo.mx"})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	if _, err := bson.Raw(doc).LookupErr("clientlimit"); err == nil {
		t.Error("una cuenta sin tope propio escribió clientlimit")
	}

	doc, err = bson.Marshal(User{Email: "ana@ejemplo.mx", ClientLimit: 500})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	value, err := bson.Raw(doc).LookupErr("clientlimit")
	if err != nil {
		t.Fatalf("no se escribió clientlimit: %v", err)
	}
	var got int64
	switch value.Type {
	case bsontype.Int32:
		got = int64(value.Int32())
	case bsontype.Int64:
		got = value.Int64()
	default:
		t.Fatalf("clientlimit se guardó como %s; se esperaba un entero", value.Type)
	}
	if got != 500 {
		t.Errorf("clientlimit = %d; se esperaba 500", got)
	}
}

// Ni el registro toma el tope del cuerpo JSON ni el tope sale en un JSON.
func TestAccountLimitIsNotJSON(t *testing.T) {
	var user User
	if err := json.Unmarshal([]byte(`{"name":"Ana","clientlimit":5000}`), &user); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if user.ClientLimit != 0 {
		t.Errorf("tope tomado del cliente: %d", user.ClientLimit)
	}
	out, err := json.Marshal(User{Name: "Ana", ClientLimit: 500})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(out), "clientlimit") {
		t.Errorf("el tope salió en el JSON: %s", out)
	}
}

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Prueba de contrato con el frontend (I-12): un registro como el que manda la app
// se decodifica, se guarda en Mongo (BSON), se vuelve a leer y sale en JSON sin
// perder ni cambiar ningún campo. Las formas vienen del frontend en origin/main:
// src/interfaces/client.ts, catalog.ts (charge y productCartItem) y payment.ts,
// y el renglón de producto como lo arman components/product/ProductItem.tsx y
// ProductoCustom.tsx, pages/clients/ClientRegister.tsx y pages/data/ClientExcel.tsx.
// Si la app agrega un campo, agrégalo aquí: la prueba falla mientras el backend
// no lo guarde.

// localOnly son los campos que el backend no guarda a propósito: cloud es el
// estado del respaldo en el teléfono y changedfields sólo viaja al subir.
var localOnly = map[string]bool{"cloud": true, "changedfields": true}

// serverAdded son los campos que agrega el servidor.
var serverAdded = map[string]bool{"backupat": true}

// Las fechas se comparan como instantes: la app manda "…:05.000Z" y Go regresa "…:05Z".
var dateFields = map[string]bool{"createat": true, "updateat": true}

const frontendClient = `{"uuid":"cli-1","name":"Ana","lastname":"López","age":40,"city":"Puebla",
	"neighborhood":"Centro","address":"5 de Mayo 10","phone":"2221234567","daywork":"Lunes",
	"cloud":0,"createat":"2026-10-20T15:04:05.000Z","updateat":"2026-10-30T15:04:05.120Z",
	"status":"active","type":"blacklist","changedfields":["type"]}`

const frontendCharge = `{"uuid":"car-1","clientuuid":"cli-1","clientname":"Ana López",
	"products":[
		{"uuid":"5b0c2a64-1d3e-4f1a-9c1b-2f7f2f8e0a11","key":"S1122","name":"Crema","page":"12","price":150.5,
			"type":"pieza","catalog":"Shelo","quantity":2,"total":301},
		{"uuid":"X1234","key":"X1234","name":"Especial","page":"nill","price":0,"type":"","catalog":"","quantity":1,"total":0},
		{"uuid":"nillCustom","name":"Adeudo Anterior","price":80,"key":"nill","page":"","quantity":1,"total":80,
			"catalog":"","type":"Deuda"},
		{"uuid":"prod-3","key":"S3000","name":"Muestra","page":"5","price":0,"type":"pieza","catalog":"Shelo",
			"quantity":0,"total":0}],
	"discount":0,"subtotal":381,"finalprice":381,"createat":"2026-10-20T15:05:00.000Z",
	"updateat":"2026-10-20T15:05:00.000Z","cloud":0,"status":"credited"}`

const frontendPayment = `{"clientuuid":"cli-1","clientname":"Ana López","uuid":"pag-1","amount":150.75,
	"method":"efectivo","concept":"abono","createat":"2026-10-20T15:07:00.000Z",
	"updateat":"2026-10-20T15:07:00.000Z","cloud":0,"status":"deleted"}`

// roundTrip hace el viaje completo de un documento: JSON de la app → modelo →
// BSON (Mongo) → modelo → JSON de bajada.
func roundTrip[T any](t *testing.T, sent string) map[string]any {
	t.Helper()
	var doc T
	if err := json.Unmarshal([]byte(sent), &doc); err != nil {
		t.Fatalf("no se decodifica lo que manda la app: %v", err)
	}
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var stored T
	if err := bson.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}
	out, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return got
}

// sameFields compara lo que mandó la app con lo que regresa el backend: cada campo
// (salvo los locales) vuelve igual y no aparece ninguno que no sea del servidor.
func sameFields(t *testing.T, where string, sent, got map[string]any) {
	t.Helper()
	for field, value := range sent {
		if localOnly[field] {
			continue
		}
		back, ok := got[field]
		if !ok {
			t.Errorf("%s: se pierde %q", where, field)
			continue
		}
		if dateFields[field] {
			if !sameInstant(value, back) {
				t.Errorf("%s: %q = %v; se mandó %v", where, field, back, value)
			}
			continue
		}
		if field == "products" {
			sentRows, _ := value.([]any)
			gotRows, _ := back.([]any)
			if len(sentRows) != len(gotRows) {
				t.Fatalf("%s: %d renglones; se mandaron %d", where, len(gotRows), len(sentRows))
			}
			for i := range sentRows {
				sameFields(t, fmt.Sprintf("%s.products[%d]", where, i), sentRows[i].(map[string]any), gotRows[i].(map[string]any))
			}
			continue
		}
		if !reflect.DeepEqual(value, back) {
			t.Errorf("%s: %q = %#v; se mandó %#v", where, field, back, value)
		}
	}
	for field := range got {
		if _, sentIt := sent[field]; !sentIt && !serverAdded[field] {
			t.Errorf("%s: aparece %q, que la app no mandó", where, field)
		}
	}
}

func sameInstant(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	if !aok || !bok {
		return false
	}
	at, err1 := time.Parse(time.RFC3339Nano, as)
	bt, err2 := time.Parse(time.RFC3339Nano, bs)
	return err1 == nil && err2 == nil && at.Equal(bt)
}

func decodeMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("JSON de prueba inválido: %v", err)
	}
	return m
}

func TestFrontendContract(t *testing.T) {
	t.Run("cliente", func(t *testing.T) {
		sameFields(t, "cliente", decodeMap(t, frontendClient), roundTrip[Client](t, frontendClient))
	})
	// Los pedidos usan el mismo modelo que los cargos
	t.Run("cargo y pedido, con sus renglones", func(t *testing.T) {
		sameFields(t, "cargo", decodeMap(t, frontendCharge), roundTrip[Charge](t, frontendCharge))
	})
	t.Run("pago", func(t *testing.T) {
		sameFields(t, "pago", decodeMap(t, frontendPayment), roundTrip[Payment](t, frontendPayment))
	})
}

// Un número donde se espera texto (celdas de Excel, productos de la versión 1.0)
// se guarda como el texto que llegó; otro tipo sigue siendo un error con el campo.
func TestTextAcceptsNumbers(t *testing.T) {
	sent := `{"uuid":"cli-2","name":"Luis","lastname":7,"city":72000,"neighborhood":15,"address":100.5,"daywork":5}`
	got := roundTrip[Client](t, sent)
	want := map[string]string{"lastname": "7", "city": "72000", "neighborhood": "15", "address": "100.5", "daywork": "5"}
	for field, value := range want {
		if got[field] != value {
			t.Errorf("%q = %#v; se esperaba el texto %q", field, got[field], value)
		}
	}

	row := roundTrip[ProductCartItem](t, `{"uuid":42,"key":1122,"name":"Crema","page":12,"type":"pieza","catalog":2025,"price":150,"quantity":1,"total":150}`)
	for field, value := range map[string]string{"uuid": "42", "key": "1122", "page": "12", "catalog": "2025"} {
		if row[field] != value {
			t.Errorf("renglón %q = %#v; se esperaba el texto %q", field, row[field], value)
		}
	}

	tests := []struct{ name, body, field string }{
		{"verdadero/falso en un texto", `{"city":true}`, "city"},
		{"objeto en un texto", `{"daywork":{"dia":"lunes"}}`, "daywork"},
		{"lista en un renglón", `{"products":[{"key":["S1"]}]}`, "products.key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Client
			var ch Charge
			var err error
			if tt.field == "products.key" {
				err = json.Unmarshal([]byte(tt.body), &ch)
			} else {
				err = json.Unmarshal([]byte(tt.body), &c)
			}
			var typeErr *json.UnmarshalTypeError
			if !errors.As(err, &typeErr) || typeErr.Field != tt.field {
				t.Errorf("error = %v; se esperaba un error de tipo en %q", err, tt.field)
			}
		})
	}

	var empty Client
	if err := json.Unmarshal([]byte(`{"city":null}`), &empty); err != nil || empty.City != "" {
		t.Errorf("null: city = %q, %v; se esperaba vacío sin error", empty.City, err)
	}
}

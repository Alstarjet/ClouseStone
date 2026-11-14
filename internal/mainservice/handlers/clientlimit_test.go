package handlers

import (
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// clientRecord es un cliente como lo manda la app, registrado el día day de
// noviembre de 2026.
func clientRecord(uuid, status string, day int) string {
	return fmt.Sprintf(`{"uuid":%q,"name":"Cliente","lastname":%q,"age":30,"city":"Mérida","neighborhood":"Centro",
		"address":"Calle 1","phone":"9990001234","daywork":"lunes","createat":"2026-11-%02dT10:00:00Z",
		"updateat":"2026-11-%02dT10:00:00Z","status":%q}`, uuid, uuid, day, day, status)
}

const limitPayment = `{"uuid":"pag-1","clientuuid":"cli-1","clientname":"Ana","amount":150,"method":"efectivo",
	"concept":"abono","createat":"2026-11-10T15:07:00Z","updateat":"2026-11-10T15:07:00Z","status":"active"}`

func backupBody(clients []string, payments ...string) string {
	return fmt.Sprintf(`{"clients":[%s],"payments":[%s],"deviceid":%q}`,
		strings.Join(clients, ","), strings.Join(payments, ","), testDeviceID)
}

func rejectedOf(t *testing.T, body []byte) []models.RejectedRecord {
	t.Helper()
	var resp models.BackupRejectedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("la respuesta no es JSON: %s", body)
	}
	return resp.Rejected
}

func sorted(list []string) []string {
	out := append([]string(nil), list...)
	sort.Strings(out)
	return out
}

// Un lote sin clientes no consulta nada del tope.
func TestClientLimitNotCheckedWithoutClients(t *testing.T) {
	store := newFakeStore("Amethyst")
	store.activeClients = 300
	rec := serveFake(store, backupBody(nil, limitPayment))
	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d (%s)", rec.Code, rec.Body)
	}
	if len(store.existingAsked) != 0 || store.activeCounted != 0 {
		t.Errorf("se consultó el tope sin clientes en el lote: %d búsquedas, %d conteos", len(store.existingAsked), store.activeCounted)
	}
}

// Los nuevos entran por fecha de registro hasta llenar los lugares; los que sobran
// se rechazan con client_limit y lo demás (pagos incluidos) se guarda.
func TestClientLimitRejectsOnlyNewClientsThatDoNotFit(t *testing.T) {
	store := newFakeStore("Amethyst")
	store.activeClients = 298 // quedan 2 lugares de 300
	clients := []string{
		clientRecord("nuevo-dia-5", "active", 5),
		clientRecord("nuevo-dia-2", "active", 2),
		clientRecord("nuevo-dia-9", "active", 9),
		clientRecord("nuevo-dia-1", "active", 1),
	}
	rec := serveFake(store, backupBody(clients, limitPayment))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código = %d; se esperaba 422 (%s)", rec.Code, rec.Body)
	}
	written := store.written()
	if got, want := sorted(written["clients"]), []string{"nuevo-dia-1", "nuevo-dia-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("clientes guardados = %v; se esperaban los dos registrados primero %v", got, want)
	}
	if !reflect.DeepEqual(written["payments"], []string{"pag-1"}) {
		t.Errorf("pagos guardados = %v; los pagos siempre pasan", written["payments"])
	}
	rejected := rejectedOf(t, rec.Body.Bytes())
	var uuids []string
	for _, r := range rejected {
		if r.Store != "clients" || r.Code != "client_limit" || !strings.Contains(r.Reason, "300") {
			t.Errorf("rechazo = %+v; se esperaba clients, client_limit y el tope en el motivo", r)
		}
		uuids = append(uuids, r.UUID)
	}
	if got, want := sorted(uuids), []string{"nuevo-dia-5", "nuevo-dia-9"}; !reflect.DeepEqual(got, want) {
		t.Errorf("rechazados = %v; se esperaban %v", got, want)
	}
}

func TestClientLimitCases(t *testing.T) {
	tests := []struct {
		name        string
		clientLimit models.AccountLimit
		active      int64
		existing    []string
		clients     []string
		wantSaved   []string
		wantCounted int
	}{
		{
			name:        "un cliente que ya está en la nube siempre se actualiza, aunque vaya pasado",
			active:      361,
			existing:    []string{"viejo"},
			clients:     []string{clientRecord("viejo", "active", 3)},
			wantSaved:   []string{"viejo"},
			wantCounted: 0,
		},
		{
			name:        "un cliente nuevo ya borrado no cuenta",
			active:      300,
			clients:     []string{clientRecord("borrado", "deleted", 3)},
			wantSaved:   []string{"borrado"},
			wantCounted: 0,
		},
		{
			name:        "con lugar para todos no rechaza nada",
			active:      10,
			clients:     []string{clientRecord("a", "active", 1), clientRecord("b", "active", 2)},
			wantSaved:   []string{"a", "b"},
			wantCounted: 1,
		},
		{
			name:        "el tope propio de la cuenta manda (500)",
			clientLimit: 500,
			active:      361,
			clients:     []string{clientRecord("a", "active", 1)},
			wantSaved:   []string{"a"},
			wantCounted: 1,
		},
		{
			name:        "ya pasado del tope: ningún nuevo entra",
			active:      361,
			existing:    []string{"viejo"},
			clients:     []string{clientRecord("viejo", "active", 3), clientRecord("nuevo", "active", 4)},
			wantSaved:   []string{"viejo"},
			wantCounted: 1,
		},
		{
			name:        "el mismo cliente dos veces en el lote ocupa un lugar",
			active:      299,
			clients:     []string{clientRecord("doble", "active", 1), clientRecord("doble", "active", 1)},
			wantSaved:   []string{"doble", "doble"},
			wantCounted: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore("Amethyst")
			store.user.ClientLimit = tt.clientLimit
			store.activeClients = tt.active
			store.existingClients = map[string]bool{}
			for _, uuid := range tt.existing {
				store.existingClients[uuid] = true
			}
			rec := serveFake(store, backupBody(tt.clients))
			if got := sorted(store.written()["clients"]); !reflect.DeepEqual(got, sorted(tt.wantSaved)) {
				t.Errorf("clientes guardados = %v; se esperaban %v (respuesta %d: %s)", got, tt.wantSaved, rec.Code, rec.Body)
			}
			if store.activeCounted != tt.wantCounted {
				t.Errorf("conteos de clientes activos = %d; se esperaban %d", store.activeCounted, tt.wantCounted)
			}
		})
	}
}

// Si la base falla al revisar el tope, 500 y nada guardado: la app reintenta el lote.
func TestClientLimitStoreErrorSavesNothing(t *testing.T) {
	store := newFakeStore("Amethyst")
	store.limitErr = errors.New("se cayó")
	rec := serveFake(store, backupBody([]string{clientRecord("a", "active", 1)}, limitPayment))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("código = %d; se esperaba 500", rec.Code)
	}
	if len(store.written()) != 0 {
		t.Errorf("se guardó algo: %v", store.written())
	}
}

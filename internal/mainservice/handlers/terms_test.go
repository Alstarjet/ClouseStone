package handlers

import (
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"strings"
	"testing"
	"time"
)

func TestTermsAcceptance(t *testing.T) {
	// Hora del servidor en otra zona: la constancia se guarda en UTC
	now := time.Date(2026, 10, 2, 10, 30, 0, 0, time.FixedZone("CST", -6*3600))

	tests := []struct {
		name        string
		body        string
		wantAt      bool
		wantVersion string
		wantErr     error // errTermsVersion, nil, o "cualquier error" si anyErr
		anyErr      bool
	}{
		{"acepta con versión", `{"email":"a@b.mx","termsaccepted":true,"termsversion":"2026-10-02"}`, true, "2026-10-02", nil, false},
		{"recorta espacios de la versión", `{"termsaccepted":true,"termsversion":"  2026-10-02 "}`, true, "2026-10-02", nil, false},
		{"cliente anterior sin la casilla", `{"email":"a@b.mx"}`, false, "", nil, false},
		{"no aceptó", `{"termsaccepted":false,"termsversion":"2026-10-02"}`, false, "", nil, false},
		{"aceptó sin versión", `{"termsaccepted":true}`, false, "", errTermsVersion, false},
		{"versión demasiado larga", `{"termsaccepted":true,"termsversion":"` + strings.Repeat("x", maxTermsVersionLength+1) + `"}`, false, "", errTermsVersion, false},
		{"tipo equivocado", `{"termsaccepted":"si","termsversion":"2026-10-02"}`, false, "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, version, err := termsAcceptance([]byte(tt.body), now)
			switch {
			case tt.anyErr:
				if err == nil || errors.Is(err, errTermsVersion) {
					t.Fatalf("err = %v; se esperaba un error de formato", err)
				}
				return
			case !errors.Is(err, tt.wantErr):
				t.Fatalf("err = %v; se esperaba %v", err, tt.wantErr)
			}
			if (at != nil) != tt.wantAt {
				t.Fatalf("fecha = %v; se esperaba fecha: %v", at, tt.wantAt)
			}
			if at != nil && (!at.Equal(now) || at.Location() != time.UTC) {
				t.Errorf("fecha = %v; se esperaba %v en UTC", at, now.UTC())
			}
			if version != tt.wantVersion {
				t.Errorf("versión = %q; se esperaba %q", version, tt.wantVersion)
			}
		})
	}
}

// El cliente no puede escribir la constancia directamente: la fecha siempre la
// pone el servidor.
func TestUserIgnoresClientAcceptanceFields(t *testing.T) {
	body := `{"name":"Ana","termsacceptedat":"2000-01-01T00:00:00Z","termsversion":"falsa"}`
	var user models.User
	if err := json.Unmarshal([]byte(body), &user); err != nil {
		t.Fatal(err)
	}
	if user.TermsAcceptedAt != nil || user.TermsVersion != "" {
		t.Errorf("constancia tomada del cliente: fecha=%v versión=%q", user.TermsAcceptedAt, user.TermsVersion)
	}
}

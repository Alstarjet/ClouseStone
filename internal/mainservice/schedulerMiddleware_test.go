package mainservice

import (
	"context"
	"errors"
	"financial-Assistant/internal/mainservice/moduls/googleid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	schedulerAudience = "https://api.ejemplo.mx/internal/plans/expire"
	schedulerAccount  = "gluzie-scheduler@proyecto.iam.gserviceaccount.com"
)

// fakeVerifier acepta sólo el token "bueno" con la audiencia esperada y regresa esos claims.
type fakeVerifier struct {
	claims       googleid.Claims
	gotAudience  string
	verifyCalled bool
}

func (f *fakeVerifier) Verify(_ context.Context, raw, audience string) (*googleid.Claims, error) {
	f.verifyCalled, f.gotAudience = true, audience
	if raw != "bueno" {
		return nil, errors.New("firma inválida")
	}
	c := f.claims
	return &c, nil
}

func TestSchedulerAuth(t *testing.T) {
	tests := []struct {
		name, audience, account, header string
		claims                          googleid.Claims
		wantStatus                      int
		wantError                       string
	}{
		{"Cloud Scheduler", schedulerAudience, schedulerAccount, "Bearer bueno",
			googleid.Claims{Email: schedulerAccount, EmailVerified: true}, http.StatusOK, ""},
		{"correo con otras mayúsculas", schedulerAudience, schedulerAccount, "Bearer bueno",
			googleid.Claims{Email: strings.ToUpper(schedulerAccount), EmailVerified: true}, http.StatusOK, ""},
		{"sin configurar la audiencia", "", schedulerAccount, "Bearer bueno",
			googleid.Claims{Email: schedulerAccount, EmailVerified: true}, http.StatusServiceUnavailable, "scheduler_not_configured"},
		{"sin configurar la cuenta", schedulerAudience, "", "Bearer bueno",
			googleid.Claims{Email: schedulerAccount, EmailVerified: true}, http.StatusServiceUnavailable, "scheduler_not_configured"},
		{"sin Authorization", schedulerAudience, schedulerAccount, "",
			googleid.Claims{}, http.StatusUnauthorized, "missing Authorization header"},
		{"sin Bearer", schedulerAudience, schedulerAccount, "bueno",
			googleid.Claims{}, http.StatusUnauthorized, "missing Authorization header"},
		{"token inválido", schedulerAudience, schedulerAccount, "Bearer malo",
			googleid.Claims{}, http.StatusUnauthorized, "invalid token"},
		{"otra cuenta de Google", schedulerAudience, schedulerAccount, "Bearer bueno",
			googleid.Claims{Email: "alguien@gmail.com", EmailVerified: true}, http.StatusForbidden, "forbidden"},
		{"correo sin verificar", schedulerAudience, schedulerAccount, "Bearer bueno",
			googleid.Claims{Email: schedulerAccount}, http.StatusForbidden, "forbidden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{claims: tt.claims}
			reached := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
			req := httptest.NewRequest(http.MethodPost, "/internal/plans/expire", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			schedulerAuth(verifier, tt.audience, tt.account, next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("estado = %d; se esperaba %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if reached != (tt.wantStatus == http.StatusOK) {
				t.Errorf("¿llegó a la ruta? %v", reached)
			}
			if tt.wantError != "" && !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Errorf("cuerpo = %s; se esperaba %q", rec.Body, tt.wantError)
			}
			if verifier.verifyCalled && verifier.gotAudience != tt.audience {
				t.Errorf("audiencia = %q; se esperaba %q", verifier.gotAudience, tt.audience)
			}
		})
	}
}

// La ruta real lee la configuración del entorno: sin ella no deja pasar a nadie.
func TestSchedulerMiddlewareNeedsConfig(t *testing.T) {
	t.Setenv("SCHEDULER_AUDIENCE", "")
	t.Setenv("SCHEDULER_SERVICE_ACCOUNT", "")
	req := httptest.NewRequest(http.MethodPost, "/internal/plans/expire", nil)
	req.Header.Set("Authorization", "Bearer bueno")
	rec := httptest.NewRecorder()
	SchedulerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("llegó a la ruta sin configuración")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("estado = %d; se esperaba 503", rec.Code)
	}
}

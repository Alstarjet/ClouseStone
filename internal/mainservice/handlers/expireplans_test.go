package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeExpirer struct {
	switched int64
	err      error
	gotNow   time.Time
}

func (f *fakeExpirer) ExpirePaidPlans(_ context.Context, now time.Time) (int64, error) {
	f.gotNow = now
	return f.switched, f.err
}

func TestExpirePlans(t *testing.T) {
	tests := []struct {
		name       string
		store      *fakeExpirer
		wantStatus int
		wantBody   string
	}{
		{"cambió tres", &fakeExpirer{switched: 3}, http.StatusOK, `{"switched":3}`},
		{"nadie vencido", &fakeExpirer{}, http.StatusOK, `{"switched":0}`},
		{"falla la base", &fakeExpirer{err: errors.New("sin conexión")}, http.StatusInternalServerError, `{"error":"expire_failed"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			rec := httptest.NewRecorder()
			ExpirePlans(tt.store).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/internal/plans/expire", nil))
			if rec.Code != tt.wantStatus || strings.TrimSpace(rec.Body.String()) != tt.wantBody {
				t.Errorf("respuesta = %d %s; se esperaba %d %s", rec.Code, rec.Body, tt.wantStatus, tt.wantBody)
			}
			if tt.store.gotNow.Before(before) || tt.store.gotNow.After(time.Now()) {
				t.Errorf("now = %v; se esperaba la hora actual", tt.store.gotNow)
			}
		})
	}
}

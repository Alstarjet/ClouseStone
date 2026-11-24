package handlers

import (
	"context"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Barrido de planes vencidos con MongoDB (se salta sin CLOUSESTONE_TEST_MONGO_URI):
// sólo baja a las cuentas de pago con la fecha ya pasada, conserva la fecha y se puede
// correr dos veces.
func TestExpirePlansWithMongo(t *testing.T) {
	db := mongotest.Connect(t)
	yesterday := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Millisecond)
	tomorrow := time.Now().Add(24 * time.Hour)

	accounts := []struct {
		email, plan string
		expires     *time.Time
		wantPlan    string
	}{
		{"vencida@ejemplo.mx", plans.Paid, &yesterday, plans.Free},
		{"vencida2@ejemplo.mx", plans.Paid, &yesterday, plans.Free},
		{"vigente@ejemplo.mx", plans.Paid, &tomorrow, plans.Paid},
		{"sinfecha@ejemplo.mx", plans.Paid, nil, plans.Paid},
		{"gratis@ejemplo.mx", plans.Free, &yesterday, plans.Free},
	}
	for _, a := range accounts {
		registerUser(t, db, a.email)
		setPlan(t, db, a.email, a.plan, a.expires)
	}

	run := func() string {
		rec := httptest.NewRecorder()
		ExpirePlans(db).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/internal/plans/expire", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("estado = %d (%s)", rec.Code, rec.Body)
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if got := run(); got != `{"switched":2}` {
		t.Errorf("primera corrida = %s; se esperaban 2", got)
	}
	if got := run(); got != `{"switched":0}` {
		t.Errorf("segunda corrida = %s; no debe cambiar a nadie", got)
	}

	users := mongotest.Raw(t, db).Collection("users")
	for _, a := range accounts {
		var doc struct {
			TypeClient    string     `bson:"typeclient"`
			PlanExpiresAt *time.Time `bson:"planexpiresat"`
		}
		if err := users.FindOne(context.Background(), bson.M{"email": a.email}).Decode(&doc); err != nil {
			t.Fatalf("%s: %v", a.email, err)
		}
		if doc.TypeClient != a.wantPlan {
			t.Errorf("%s: plan = %q; se esperaba %q", a.email, doc.TypeClient, a.wantPlan)
		}
		if (a.expires == nil) != (doc.PlanExpiresAt == nil) ||
			(a.expires != nil && !doc.PlanExpiresAt.Equal(a.expires.Truncate(time.Millisecond))) {
			t.Errorf("%s: planexpiresat = %v; debe conservarse (%v)", a.email, doc.PlanExpiresAt, a.expires)
		}
	}
}

// Un pago que alarga la vigencia gana: con la fecha en el futuro, el barrido ya no la toca.
func TestExpirePlansSkipsRenewed(t *testing.T) {
	db := mongotest.Connect(t)
	nextYear := time.Now().AddDate(1, 0, 0)
	registerUser(t, db, "renovo@ejemplo.mx")
	setPlan(t, db, "renovo@ejemplo.mx", plans.Paid, &nextYear)

	switched, err := db.ExpirePaidPlans(context.Background(), time.Now())
	if err != nil || switched != 0 {
		t.Fatalf("switched = %d, err = %v; se esperaba 0", switched, err)
	}
}

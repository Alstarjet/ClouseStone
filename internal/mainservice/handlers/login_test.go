package handlers

import (
	"context"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// fakeAccount es la base en memoria del login y de /GetData: regresa un documento
// por colección, anota las consultas y falla la que se le diga.
type fakeAccount struct {
	failing string
	queries map[string][]any
}

func (f *fakeAccount) query(collection string, filter any) error {
	if f.queries == nil {
		f.queries = map[string][]any{}
	}
	f.queries[collection] = append(f.queries[collection], filter)
	if f.failing == collection {
		return errors.New("corte momentáneo de Atlas")
	}
	return nil
}

func (f *fakeAccount) FindAllClients(_ context.Context, filter interface{}) ([]models.Client, error) {
	if err := f.query("clients", filter); err != nil {
		return nil, err
	}
	return []models.Client{{UUID: "cli-1"}}, nil
}

func (f *fakeAccount) FindAllCharges(_ context.Context, filter interface{}) ([]models.Charge, error) {
	if err := f.query("charges", filter); err != nil {
		return nil, err
	}
	return []models.Charge{{UUID: "car-1"}}, nil
}

func (f *fakeAccount) FindAllOrders(_ context.Context, filter interface{}) ([]models.Charge, error) {
	if err := f.query("orders", filter); err != nil {
		return nil, err
	}
	return []models.Charge{{UUID: "ped-1"}}, nil
}

func (f *fakeAccount) FindAllPayments(_ context.Context, filter interface{}) ([]models.Payment, error) {
	if err := f.query("payments", filter); err != nil {
		return nil, err
	}
	return []models.Payment{{UUID: "pag-1"}}, nil
}

// I-09: si falla cualquiera de las cuatro consultas, el login no responde con una
// parte de los datos (la cuenta se vería vacía o incompleta en el teléfono nuevo).
func TestConsultDataForNewDevice(t *testing.T) {
	user := models.User{ID: primitive.NewObjectID()}

	t.Run("todo se leyó", func(t *testing.T) {
		db := &fakeAccount{}
		data, err := consultDataForNewDevice(context.Background(), db, user)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if len(data.Clients) != 1 || len(data.Charges) != 1 || len(data.Orders) != 1 || len(data.Payments) != 1 {
			t.Errorf("datos = %+v", data)
		}
		want := bson.D{{Key: "usermongoid", Value: user.ID.Hex()}}
		for coll, filters := range db.queries {
			if len(filters) != 1 || !reflect.DeepEqual(filters[0], want) {
				t.Errorf("%s: consultas = %v; se esperaba sólo las del usuario", coll, filters)
			}
		}
	})
	for _, failing := range []string{"payments", "charges", "clients", "orders"} {
		t.Run("falla "+failing, func(t *testing.T) {
			data, err := consultDataForNewDevice(context.Background(), &fakeAccount{failing: failing}, user)
			if err == nil {
				t.Fatalf("se esperaba error; datos = %+v", data)
			}
			if !reflect.DeepEqual(data, models.AllData{}) {
				t.Errorf("con error no se regresan datos: %+v", data)
			}
		})
	}
}

// /GetData (legado): una consulta por colección, sólo del usuario, cada documento
// una vez; nunca un documento vacío por un id que no existe.
func TestConsutDocumentsForDevice(t *testing.T) {
	ids := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID()}
	device := models.Device{
		UUID:      "tel-1",
		ClientIDs: []string{ids[0].Hex(), "no-es-un-id", ids[0].Hex(), ids[1].Hex()},
		// cargos, pedidos y pagos sin pendientes: no se consultan
	}
	db := &fakeAccount{}
	data, err := ConsutDocumentsForDevice(context.Background(), db, device, "usuario-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(data.Clients) != 1 || data.Charges != nil || data.Orders != nil || data.Payments != nil {
		t.Errorf("datos = %+v", data)
	}
	wantFilter := bson.D{
		{Key: "usermongoid", Value: "usuario-1"},
		{Key: "_id", Value: bson.D{{Key: "$in", Value: []primitive.ObjectID{ids[0], ids[0], ids[1]}}}},
	}
	if got := db.queries["clients"]; len(got) != 1 || !reflect.DeepEqual(got[0], wantFilter) {
		t.Errorf("consulta de clientes = %v; se esperaba %v", got, wantFilter)
	}
	if len(db.queries) != 1 {
		t.Errorf("consultas = %v; sólo había pendientes de clientes", db.queries)
	}

	_, err = ConsutDocumentsForDevice(context.Background(), &fakeAccount{failing: "clients"}, device, "usuario-1")
	if err == nil || !strings.Contains(err.Error(), "corte") {
		t.Errorf("error = %v; un error de la base no se ignora", err)
	}
}

// finderFunc es un userFinder en memoria.
type finderFunc func(email string) (models.User, error)

func (f finderFunc) FindUser(_ context.Context, email string) (models.User, error) { return f(email) }

// I-09: si la base falla al buscar la cuenta, el login responde 500 («intenta de
// nuevo»), no 401 «usuario o contraseña incorrectos».
func TestAuthenticateUserDatabaseError(t *testing.T) {
	hashed, err := bcrypt.GenerateFromPassword([]byte("clave-de-prueba-1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	account := models.User{ID: primitive.NewObjectID(), Email: "ana@ejemplo.mx", Password: string(hashed)}
	tests := []struct {
		name     string
		finder   finderFunc
		password string
		wantCode int
		wantBody string
	}{
		{"la base falla", func(string) (models.User, error) { return models.User{}, errors.New("server selection timeout") },
			"clave-de-prueba-1", http.StatusInternalServerError, `{"error":"internal server error"}`},
		{"la cuenta no existe", func(string) (models.User, error) { return models.User{}, mongo.ErrNoDocuments },
			"clave-de-prueba-1", http.StatusUnauthorized, `{"error":"usuario o contraseña incorrectos"}`},
		{"contraseña incorrecta", func(string) (models.User, error) { return account, nil },
			"otra-clave", http.StatusUnauthorized, `{"error":"usuario o contraseña incorrectos"}`},
		{"todo bien", func(string) (models.User, error) { return account, nil },
			"clave-de-prueba-1", http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"email":"ana@ejemplo.mx","password":%q,"device":"dispositivo-1"}`, tt.password)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
			user, _, err := authenticateUser(context.Background(), tt.finder, rec, req)
			if tt.wantCode == http.StatusOK {
				if err != nil || user.ID != account.ID || rec.Body.Len() != 0 {
					t.Errorf("usuario = %+v, error = %v, respuesta = %s", user, err, rec.Body)
				}
				return
			}
			if err == nil || rec.Code != tt.wantCode {
				t.Fatalf("código = %d, error = %v; se esperaba %d", rec.Code, err, tt.wantCode)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("cuerpo = %s; se esperaba %s", got, tt.wantBody)
			}
		})
	}
}

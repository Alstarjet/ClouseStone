package devices

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Necesitan MongoDB (ver mongotest): se saltan sin CLOUSESTONE_TEST_MONGO_URI.

func newUser() models.User {
	return models.User{ID: primitive.NewObjectID(), Name: "Ana", LastName: "López", Email: "ana@ejemplo.mx"}
}

// login registra el dispositivo como lo hace /login.
func login(t *testing.T, db *database.MongoClient, user models.User, deviceUUID, token string) {
	t.Helper()
	if err := AddDeviceAndRefreshToken(context.Background(), db, user, token, time.Now().Add(time.Hour), deviceUUID); err != nil {
		t.Fatalf("AddDeviceAndRefreshToken(%s): %v", deviceUUID, err)
	}
}

// tokens regresa el hash del refresh token de cada entrada de dispositivo (un
// dispositivo repetido aparece dos veces).
func tokens(t *testing.T, db *database.MongoClient, user models.User) map[string][]string {
	t.Helper()
	doc, err := db.FindDevice(context.Background(), bson.D{{Key: "_id", Value: user.ID}})
	if err != nil {
		t.Fatalf("FindDevice: %v", err)
	}
	if doc.UserMongoID != user.ID.Hex() || doc.UserEmail != user.Email {
		t.Errorf("documento de dispositivos = %+v", doc)
	}
	out := map[string][]string{}
	for _, d := range doc.Devices {
		out[d.UUID] = append(out[d.UUID], d.Refreshtoken.Token)
	}
	return out
}

func TestAddDeviceAndRefreshToken(t *testing.T) {
	db := mongotest.Connect(t)

	t.Run("primer login: crea el documento con el dispositivo", func(t *testing.T) {
		user := newUser()
		login(t, db, user, "A", "token-A")
		if got := tokens(t, db, user); len(got) != 1 || got["A"][0] != utilities.HashToken("token-A") {
			t.Errorf("dispositivos = %v", got)
		}
	})

	t.Run("volver a entrar en el mismo dispositivo sólo cambia su token", func(t *testing.T) {
		user := newUser()
		login(t, db, user, "A", "token-viejo")
		login(t, db, user, "B", "token-B")
		login(t, db, user, "A", "token-nuevo")
		got := tokens(t, db, user)
		if len(got["A"]) != 1 || got["A"][0] != utilities.HashToken("token-nuevo") {
			t.Errorf("A = %v; se esperaba una sola entrada con el token nuevo", got["A"])
		}
		if len(got["B"]) != 1 || got["B"][0] != utilities.HashToken("token-B") {
			t.Errorf("B = %v; entrar en A no debe tocar el token de B", got["B"])
		}
	})

	// Antes: los dos leían el documento, ninguno veía el dispositivo y los dos lo
	// agregaban; con dos entradas, /GetJwt comparaba contra la primera y cerraba la sesión.
	t.Run("logins a la vez del mismo dispositivo no lo repiten", func(t *testing.T) {
		user := newUser()
		const logins = 8
		var wg sync.WaitGroup
		errs := make(chan error, logins)
		start := make(chan struct{})
		for i := 0; i < logins; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				errs <- AddDeviceAndRefreshToken(context.Background(), db, user, fmt.Sprintf("token-%d", i), time.Now().Add(time.Hour), "A")
			}(i)
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Errorf("login: %v", err)
			}
		}
		if got := tokens(t, db, user); len(got) != 1 || len(got["A"]) != 1 {
			t.Errorf("dispositivos = %v; se esperaba una sola entrada de A", got)
		}
	})
}

// /DeleteIds (legado) vacía sólo la cola de ese dispositivo, sin tocar su token
// ni nada de los demás.
func TestDeleteIDsForDevice(t *testing.T) {
	db := mongotest.Connect(t)
	user := newUser()
	ctx := context.Background()
	login(t, db, user, "A", "token-A")
	login(t, db, user, "B", "token-B")
	// La cola vieja: ya nadie la escribe, pero puede traer ids de antes
	_, err := mongotest.Raw(t, db).Collection("devices").UpdateOne(ctx, bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{"devices.$[].clientids": bson.A{"c1", "c2"}, "devices.$[].paymentids": bson.A{"p1"}}})
	if err != nil {
		t.Fatalf("preparar la cola: %v", err)
	}

	if err := DeleteIDsForDevice(ctx, db, user, "A"); err != nil {
		t.Fatalf("DeleteIDsForDevice: %v", err)
	}
	doc, err := db.FindDevice(ctx, bson.D{{Key: "_id", Value: user.ID}})
	if err != nil {
		t.Fatalf("FindDevice: %v", err)
	}
	for _, d := range doc.Devices {
		switch d.UUID {
		case "A":
			if len(d.ClientIDs)+len(d.PaymentIDs)+len(d.ChargeIDs)+len(d.OrderIDs) != 0 {
				t.Errorf("A: cola = %+v; se esperaba vacía", d)
			}
		case "B":
			if len(d.ClientIDs) != 2 || len(d.PaymentIDs) != 1 {
				t.Errorf("B: cola = %+v; no se debía tocar", d)
			}
		}
		if d.Refreshtoken.Token != utilities.HashToken("token-"+d.UUID) {
			t.Errorf("%s: se perdió el refresh token", d.UUID)
		}
	}

	pending, err := ConsultIDs(ctx, db, user, "B")
	if err != nil || len(pending.ClientIDs) != 2 {
		t.Errorf("ConsultIDs(B) = %+v, %v", pending, err)
	}
	if none, err := ConsultIDs(ctx, db, user, "no-registrado"); err != nil || none.UUID != "" {
		t.Errorf("ConsultIDs de un dispositivo que no existe = %+v, %v; se esperaba vacío", none, err)
	}
}

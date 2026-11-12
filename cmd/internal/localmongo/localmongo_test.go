package localmongo

import (
	"errors"
	"testing"
)

func TestRequireLocal(t *testing.T) {
	tests := []struct {
		name, uri string
		wantLocal bool
	}{
		{"localhost", "mongodb://localhost:27017", true},
		{"localhost sin puerto", "mongodb://localhost", true},
		{"con usuario, base y opciones", "mongodb://usuario:clave@localhost:27017/gluzie_local?authSource=admin", true},
		{"127.0.0.1", "mongodb://127.0.0.1:27017", true},
		{"otra IP de loopback", "mongodb://127.0.0.2:27017", true},
		{"::1", "mongodb://[::1]:27017", true},
		{"nombre .localhost", "mongodb://mongo.localhost:27017", true},
		{"socket local", "mongodb://%2Ftmp%2Fmongodb-27017.sock", true},
		{"varios hosts locales", "mongodb://localhost:27017,localhost:27018/?replicaSet=rs0", true},
		{"mayúsculas", "mongodb://LOCALHOST:27017", true},

		{"Atlas (mongodb+srv)", "mongodb+srv://usuario:clave@cloudstone.ab1cd.mongodb.net/CloudStoneProd", false},
		{"mongodb+srv en mayúsculas", "MONGODB+SRV://cloudstone.ab1cd.mongodb.net", false},
		{"host de Atlas sin srv", "mongodb://ac-abc-shard-00-00.ab1cd.mongodb.net:27017", false},
		{"IP de otra computadora", "mongodb://192.168.1.20:27017", false},
		{"uno de los hosts no es local", "mongodb://localhost:27017,10.0.0.5:27017", false},
		{"todas las interfaces no es local", "mongodb://0.0.0.0:27017", false},
		{"nombre que sólo empieza con localhost", "mongodb://localhost.ejemplo.mx:27017", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireLocal(tt.uri)
			if tt.wantLocal {
				if err != nil {
					t.Errorf("error inesperado: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotLocal) {
				t.Errorf("error = %v; se esperaba ErrNotLocal", err)
			}
		})
	}

	if err := RequireLocal("localhost:27017"); err == nil || errors.Is(err, ErrNotLocal) {
		t.Errorf("URI sin esquema: error = %v; se esperaba URI inválida", err)
	}
}

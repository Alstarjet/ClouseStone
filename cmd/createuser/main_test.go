package main

import (
	"errors"
	"financial-Assistant/cmd/internal/localmongo"
	"testing"
	"time"
)

// I-15: createuser se niega a correr contra una base que no es local, antes de
// conectarse a nada.
func TestRunRefusesRemoteDatabases(t *testing.T) {
	for _, tt := range []struct{ name, uri string }{
		{"Atlas", "mongodb+srv://usuario:clave@cloudstone.ab1cd.mongodb.net"},
		// Remota y con GSSAPI (no viene compilado): si faltara la protección, el
		// driver fallaría al configurarse, sin salir a la red.
		{"IP remota", "mongodb://usuario@203.0.113.10:27017/?authMechanism=GSSAPI"},
	} {
		uri := tt.uri
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			err := run([]string{"-uri", uri, "-db", "CloudStoneProd", "-email", "ana@ejemplo.mx", "-password", "clave-de-prueba-1"})
			if !errors.Is(err, localmongo.ErrNotLocal) {
				t.Fatalf("error = %v; se esperaba ErrNotLocal", err)
			}
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Errorf("tardó %v: parece que intentó conectarse", elapsed)
			}
		})
	}
}

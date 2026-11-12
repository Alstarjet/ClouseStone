package main

import (
	"errors"
	"financial-Assistant/cmd/internal/localmongo"
	"path/filepath"
	"testing"
	"time"
)

// I-15: seed (que con -drop borra la colección) se niega a correr contra una base
// que no es local, antes de leer el archivo o conectarse a nada.
func TestRunRefusesRemoteDatabases(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-existe.json")
	for _, tt := range []struct{ name, uri string }{
		{"Atlas", "mongodb+srv://usuario:clave@cloudstone.ab1cd.mongodb.net"},
		// Remota y con GSSAPI (no viene compilado): si faltara la protección, el
		// driver fallaría al configurarse, sin salir a la red.
		{"IP remota", "mongodb://usuario@203.0.113.10:27017/?authMechanism=GSSAPI"},
	} {
		uri := tt.uri
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			err := run([]string{"-uri", uri, "-db", "CloudStoneProd", "-file", missing, "-collection", "clients", "-drop"})
			if !errors.Is(err, localmongo.ErrNotLocal) {
				t.Fatalf("error = %v; se esperaba ErrNotLocal (antes de leer el archivo)", err)
			}
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Errorf("tardó %v: parece que intentó conectarse", elapsed)
			}
		})
	}
}

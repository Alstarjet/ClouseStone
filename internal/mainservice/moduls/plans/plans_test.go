package plans

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestFor(t *testing.T) {
	free := Rules{CloudBackup: false, ClientLimit: 50}
	tests := []struct {
		name       string
		typeclient string
		want       Rules
		wantLog    bool
	}{
		{"versión gratis", "Quartz", free, false},
		{"plan de pago", "Amethyst", Rules{CloudBackup: true, ClientLimit: 200}, false},
		{"plan vacío: como la versión gratis", "", free, true},
		{"plan desconocido: como la versión gratis", "Diamond", free, true},
		{"con otras mayúsculas es otro plan", "amethyst", free, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&out)
			t.Cleanup(func() { log.SetOutput(previous) })

			if got := For(tt.typeclient); got != tt.want {
				t.Errorf("For(%q) = %+v; se esperaba %+v", tt.typeclient, got, tt.want)
			}
			if logged := strings.Contains(out.String(), "plan desconocido"); logged != tt.wantLog {
				t.Errorf("¿se anotó en el log? = %v; se esperaba %v (log: %q)", logged, tt.wantLog, out.String())
			}
		})
	}
}

func TestPlanNames(t *testing.T) {
	if Free != "Quartz" || Paid != "Amethyst" {
		t.Errorf("Free = %q, Paid = %q: son los typeclient que ya están en la base", Free, Paid)
	}
}

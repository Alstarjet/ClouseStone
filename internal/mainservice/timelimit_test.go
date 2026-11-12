package mainservice

import (
	"bufio"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// El handler recibe el tiempo límite de la ruta en su contexto (con él consulta
// y escribe en Mongo).
func TestWithTimeLimitSetsDeadline(t *testing.T) {
	var deadline time.Time
	var ok bool
	handler := withTimeLimit(45*time.Second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok = r.Context().Deadline()
	}))
	start := time.Now()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/UploadData", nil))
	if !ok {
		t.Fatal("el contexto no trae tiempo límite")
	}
	if left := deadline.Sub(start); left < 44*time.Second || left > 46*time.Second {
		t.Errorf("tiempo límite = %v; se esperaban 45 s", left)
	}
}

// slowServer levanta el router (o un handler) con plazos de servidor cortos, como
// los 15 s de main.go pero en milisegundos.
func slowServer(t *testing.T, handler http.Handler, readTimeout, writeTimeout time.Duration) string {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ReadTimeout = readTimeout
	srv.Config.WriteTimeout = writeTimeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

// Con su propio tiempo límite, una respuesta que tarda más que el WriteTimeout
// del servidor sí llega (antes: el lote se guardaba pero el teléfono recibía un
// error y lo repetía para siempre, I-03).
func TestWithTimeLimitOutlivesServerTimeouts(t *testing.T) {
	const serverTimeout = 300 * time.Millisecond
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * serverTimeout)
		w.Write([]byte("guardado"))
	})
	tests := []struct {
		name    string
		handler http.Handler
		wantOK  bool
	}{
		{"sin tiempo límite propio: la respuesta se pierde", slow, false},
		{"con tiempo límite propio: la respuesta llega", withTimeLimit(10*time.Second, slow), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := slowServer(t, tt.handler, serverTimeout, serverTimeout)
			resp, err := http.Get("http://" + addr + "/")
			if err == nil {
				defer resp.Body.Close()
			}
			var body []byte
			if err == nil {
				body, err = io.ReadAll(resp.Body)
			}
			got := err == nil && resp.StatusCode == http.StatusOK && string(body) == "guardado"
			if got != tt.wantOK {
				t.Errorf("¿llegó la respuesta? = %v (err: %v); se esperaba %v", got, err, tt.wantOK)
			}
		})
	}
}

// postSlowly manda un POST cuyo cuerpo termina de llegar después de pause, como
// un lote pesado por una red de celular lenta, y regresa el código y el cuerpo de
// la respuesta ("" si no hubo respuesta).
func postSlowly(t *testing.T, addr, path, authorization, body string, pause time.Duration) (int, string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer conn.Close()
	head := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n", path, addr, len(body))
	if authorization != "" {
		head += "Authorization: " + authorization + "\r\n"
	}
	half := len(body) / 2
	if _, err := io.WriteString(conn, head+"\r\n"+body[:half]); err != nil {
		t.Fatalf("mandar: %v", err)
	}
	time.Sleep(pause)
	if _, err := io.WriteString(conn, body[half:]); err != nil {
		return 0, "" // el servidor ya cerró
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(data))
}

// /UploadData y /login tienen su propio tiempo límite: su cuerpo puede tardar más
// que el ReadTimeout del servidor en llegar. Las demás rutas siguen con el del
// servidor. Sin Mongo: los cuerpos son JSON inválidos, así que el handler responde
// 400 «formato inválido» en cuanto termina de leerlos, antes de tocar la base.
func TestRoutesWithOwnTimeLimit(t *testing.T) {
	t.Setenv("ACCESS_SECRET", testAccessSecret)
	t.Setenv("ENVIRONMENT", "local")
	user := models.User{ID: primitive.NewObjectID(), Email: "ana@ejemplo.mx"}
	token, _, err := utilities.GenerateToken(user, "dispositivo-1", testAccessSecret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	const serverTimeout = 300 * time.Millisecond
	addr := slowServer(t, NewRouter(&Server{}), serverTimeout, 0)
	notJSON := `{"esto no es": JSON` + strings.Repeat(" ", 64)

	const readAll = `{"error":"invalid request format"}` // leyó el cuerpo completo
	for _, path := range []string{"/UploadData", "/login", "/loginForce"} {
		t.Run(path+" lee el cuerpo completo", func(t *testing.T) {
			code, body := postSlowly(t, addr, path, "Bearer "+token, notJSON, 3*serverTimeout)
			if code != http.StatusBadRequest || body != readAll {
				t.Errorf("respuesta = %d %s; se esperaba 400 %s", code, body, readAll)
			}
		})
	}
	// Control: una ruta sin tiempo límite propio no alcanza a leer el cuerpo (si
	// alguna vez lo leyera, respondería «formato de solicitud inválido»).
	t.Run("/register sigue con el plazo del servidor", func(t *testing.T) {
		code, body := postSlowly(t, addr, "/register", "", notJSON, 3*serverTimeout)
		if code != 0 && body != `{"error":"error al leer la solicitud"}` {
			t.Errorf("respuesta = %d %s; se esperaba que el plazo del servidor cortara la lectura", code, body)
		}
	})
}

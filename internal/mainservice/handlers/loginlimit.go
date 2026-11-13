package handlers

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Freno de intentos de /login (evaluación de seguridad del 12-nov-2026, S-03).
// Sin él se podían probar contraseñas sin límite, y cada intento contra un correo
// real cuesta decenas de milisegundos de CPU (bcrypt).
//
//   - Por cuenta: 10 intentos fallidos en 15 minutos, vengan de donde vengan. Frena
//     a quien reparte los intentos entre muchas IP.
//   - Por IP: 20 intentos fallidos en 15 minutos, contra la cuenta que sea. Frena
//     a quien prueba muchas cuentas desde un mismo lugar.
//
// Pasado el tope, /login responde 429 sin tocar la base ni bcrypt, también con la
// contraseña correcta, hasta que termine la ventana. Un login bueno borra los
// fallos de esa cuenta.
//
// La cuenta vive en la memoria de cada instancia: con varias instancias de Cloud
// Run el tope real es el de cada una, y se pierde al reiniciar. Es suficiente
// para adivinar contraseñas (10 intentos por cuarto de hora no alcanzan para
// nada); el freno de volumen va delante del servicio.
const (
	loginWindow          = 15 * time.Minute
	maxLoginFailsAccount = 10
	maxLoginFailsIP      = 20
	// maxLimiterKeys acota la memoria de cada contador (S-02: nada crece sin tope).
	maxLimiterKeys = 20000
)

type failCount struct {
	count   int
	resetAt time.Time
}

// attemptLimiter cuenta los fallos de cada llave dentro de una ventana fija.
type attemptLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	now     func() time.Time
	entries map[string]*failCount
}

func newAttemptLimiter(max int, window time.Duration) *attemptLimiter {
	return &attemptLimiter{max: max, window: window, now: time.Now, entries: map[string]*failCount{}}
}

// retryAfter dice cuánto falta para que la llave pueda volver a intentar; 0 si
// puede ya.
func (l *attemptLimiter) retryAfter(key string) time.Duration {
	if key == "" {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok {
		return 0
	}
	now := l.now()
	if !now.Before(entry.resetAt) {
		delete(l.entries, key)
		return 0
	}
	if entry.count < l.max {
		return 0
	}
	return entry.resetAt.Sub(now)
}

// fail anota un fallo de la llave. Regresa true cuando con éste llegó al tope.
func (l *attemptLimiter) fail(key string) bool {
	if key == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, ok := l.entries[key]
	if ok && now.Before(entry.resetAt) {
		entry.count++
		return entry.count == l.max
	}
	if !ok && len(l.entries) >= maxLimiterKeys {
		l.makeRoom(now)
	}
	l.entries[key] = &failCount{count: 1, resetAt: now.Add(l.window)}
	return l.max == 1
}

// makeRoom quita las ventanas vencidas y, si aun así no cabe, empieza de cero:
// prefiere olvidar fallos a crecer sin tope.
func (l *attemptLimiter) makeRoom(now time.Time) {
	for key, entry := range l.entries {
		if !now.Before(entry.resetAt) {
			delete(l.entries, key)
		}
	}
	if len(l.entries) >= maxLimiterKeys {
		log.Printf("login: el contador de intentos llegó a %d llaves; se reinicia", len(l.entries))
		l.entries = map[string]*failCount{}
	}
}

func (l *attemptLimiter) forget(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// loginLimiter junta los dos contadores de /login.
type loginLimiter struct {
	byIP      *attemptLimiter
	byAccount *attemptLimiter
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		byIP:      newAttemptLimiter(maxLoginFailsIP, loginWindow),
		byAccount: newAttemptLimiter(maxLoginFailsAccount, loginWindow),
	}
}

// loginLimits es el freno de /login y /loginForce (comparten cuenta).
var loginLimits = newLoginLimiter()

// blocked dice cuánto falta para que esa IP o esa cuenta puedan intentar; 0 si
// pueden ya.
func (l *loginLimiter) blocked(ip, email string) time.Duration {
	wait := l.byIP.retryAfter(ip)
	if w := l.byAccount.retryAfter(email); w > wait {
		wait = w
	}
	return wait
}

// failed anota un intento fallido. email vacío: sólo cuenta para la IP.
func (l *loginLimiter) failed(ip, email string) {
	if l.byIP.fail(ip) {
		log.Printf("login: tope de intentos fallidos por IP alcanzado desde %s", ip)
	}
	if l.byAccount.fail(email) {
		// Sin el correo, para no dejarlo en los logs
		log.Printf("login: tope de intentos fallidos de una cuenta alcanzado; el último, desde %s", ip)
	}
}

func (l *loginLimiter) succeeded(email string) {
	l.byAccount.forget(email)
}

// tooManyAttempts responde el 429 del freno, con los segundos que faltan.
func tooManyAttempts(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait/time.Second) + 1
	w.Header().Set("Retry-After", fmt.Sprint(seconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too_many_attempts", "retryafter": seconds})
}

// rateLimitIP es la IP con la que se cuentan los intentos.
//
// Cloud Run agrega al final de X-Forwarded-For la IP de quien se conectó; lo que
// venga antes lo escribió el cliente y puede ser inventado. Por eso se toma la
// ÚLTIMA: con la primera, cada intento podría traer una IP distinta y el freno no
// serviría. Si algún día hay un balanceador o un proxy delante del servicio, la
// última será la suya y hay que cambiar esto.
//
// De una IPv6 se usa su red /64: quien tiene una, tiene las 2^64 direcciones.
func rateLimitIP(r *http.Request) string {
	host := ""
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		host = strings.TrimSpace(parts[len(parts)-1])
	}
	if host == "" {
		var err error
		if host, _, err = net.SplitHostPort(r.RemoteAddr); err != nil {
			host = r.RemoteAddr
		}
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return host
	}
	addr = addr.Unmap()
	if addr.Is6() {
		if prefix, err := addr.Prefix(64); err == nil {
			return prefix.String()
		}
	}
	return addr.String()
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// burnPasswordCheck gasta lo mismo que revisar una contraseña. Se usa cuando el
// correo no existe, para que /login tarde igual y no delate qué correos tienen
// cuenta.
func burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("sin cuenta"), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

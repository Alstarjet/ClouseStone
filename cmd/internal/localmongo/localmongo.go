// Package localmongo protege las herramientas de desarrollo de cmd/ para que sólo
// corran contra un MongoDB de esta computadora (I-15). Corridas contra
// producción, createuser regresa una cuenta a la versión gratis y le borra su
// vigencia, y seed -drop borra colecciones completas.
package localmongo

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

// ErrNotLocal indica que la URI apunta a un servidor que no es esta computadora.
var ErrNotLocal = errors.New("sólo se puede usar contra un MongoDB local (localhost, 127.0.0.1 o ::1)")

// RequireLocal regresa un error si la URI no es de un MongoDB local: un
// mongodb+srv:// (siempre es un clúster en la nube, como Atlas) o cualquier host
// que no sea localhost, una IP de loopback o un socket local. No se conecta ni
// consulta DNS.
func RequireLocal(uri string) error {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(uri)), connstring.SchemeMongoDBSRV+"://") {
		return fmt.Errorf("%w: mongodb+srv:// es un clúster en la nube", ErrNotLocal)
	}
	// Con mongodb:// el parser no consulta DNS
	cs, err := connstring.Parse(uri)
	if err != nil {
		return fmt.Errorf("URI de MongoDB inválida: %w", err)
	}
	if len(cs.Hosts) == 0 {
		return fmt.Errorf("%w: la URI no trae servidor", ErrNotLocal)
	}
	for _, host := range cs.Hosts {
		if !isLocal(host) {
			return fmt.Errorf("%w: %q no es local", ErrNotLocal, host)
		}
	}
	return nil
}

// isLocal dice si un host (con o sin puerto) es esta computadora.
func isLocal(hostport string) bool {
	if strings.HasSuffix(hostport, ".sock") {
		return true // socket de Unix
	}
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

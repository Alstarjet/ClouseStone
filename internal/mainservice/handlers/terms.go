package handlers

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// maxTermsVersionLength acota la versión de los documentos legales que manda el
// cliente (hoy es una fecha, p. ej. "2026-10-02").
const maxTermsVersionLength = 40

// errTermsVersion indica que se aceptaron los documentos sin una versión válida.
var errTermsVersion = errors.New("versión de los términos inválida")

// termsAcceptance lee del cuerpo del registro la aceptación de los Términos, el
// Aviso de privacidad y la Política de cookies (campos termsaccepted y
// termsversion). Si el usuario los aceptó, devuelve la fecha de aceptación (la
// del servidor, nunca una que mande el cliente) y la versión aceptada.
//
// Si el cuerpo no trae la aceptación devuelve nil, sin error: los clientes que
// todavía no tienen la casilla se siguen registrando, sólo que sin constancia.
func termsAcceptance(body []byte, now time.Time) (*time.Time, string, error) {
	var req struct {
		Accepted bool   `json:"termsaccepted"`
		Version  string `json:"termsversion"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, "", err
	}
	if !req.Accepted {
		return nil, "", nil
	}
	version := strings.TrimSpace(req.Version)
	if version == "" || len(version) > maxTermsVersionLength {
		return nil, "", errTermsVersion
	}
	acceptedAt := now.UTC()
	return &acceptedAt, version, nil
}

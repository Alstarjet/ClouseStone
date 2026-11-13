package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Revisión de que quien se registra es una persona y no un bot: una trampa en el
// formulario y Cloudflare Turnstile (https://developers.cloudflare.com/turnstile/).
//
//   - Trampa: el formulario del registro trae un campo escondido, "website", que una
//     persona nunca ve ni llena. Si llega con algo, es un bot.
//   - Turnstile: el formulario muestra el widget de Cloudflare y manda la prueba que
//     entrega en "turnstiletoken". Aquí se le pregunta a Cloudflare si es buena (cada
//     prueba sirve una sola vez). Sólo se revisa si el servicio tiene la variable
//     TURNSTILE_SECRET_KEY: sin ella (local, o mientras se publica el frontend que manda
//     la prueba) el registro funciona como antes.

// turnstileSecretEnv es la variable con la llave secreta de Turnstile.
const turnstileSecretEnv = "TURNSTILE_SECRET_KEY"

// turnstileVerifyURL es el servicio de Cloudflare que revisa la prueba (variable
// para poder usar uno de mentiras en las pruebas).
var turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// turnstileClient no espera a Cloudflare más de unos segundos.
var turnstileClient = &http.Client{Timeout: 8 * time.Second}

// maxTurnstileTokenLength es lo más largo que acepta Cloudflare.
const maxTurnstileTokenLength = 2048

var (
	// errNotHuman: la trampa trae algo, falta la prueba o Cloudflare la rechazó.
	errNotHuman = errors.New("no se pudo confirmar que no es un bot")
	// errHumanCheckUnavailable: no se pudo preguntar a Cloudflare (red, caída).
	errHumanCheckUnavailable = errors.New("no se pudo revisar la prueba de Turnstile")
)

// humanCheck revisa la trampa y, si el servicio tiene la llave secreta, la prueba de
// Turnstile del cuerpo del registro. remoteIP es la IP de quien se registra (opcional:
// Cloudflare la usa como dato extra).
func humanCheck(ctx context.Context, body []byte, remoteIP string) error {
	var req struct {
		Website string `json:"website"`
		Token   string `json:"turnstiletoken"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return errNotHuman
	}
	if strings.TrimSpace(req.Website) != "" {
		return errNotHuman
	}
	secret := strings.TrimSpace(os.Getenv(turnstileSecretEnv))
	if secret == "" {
		return nil
	}
	token := strings.TrimSpace(req.Token)
	if token == "" || len(token) > maxTurnstileTokenLength {
		return errNotHuman
	}

	form := url.Values{"secret": {secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: %v", errHumanCheckUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := turnstileClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", errHumanCheckUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: Cloudflare respondió %d", errHumanCheckUnavailable, resp.StatusCode)
	}
	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: respuesta ilegible: %v", errHumanCheckUnavailable, err)
	}
	if !result.Success {
		// La llave secreta equivocada no es culpa de quien se registra
		for _, code := range result.ErrorCodes {
			if code == "invalid-input-secret" || code == "missing-input-secret" {
				return fmt.Errorf("%w: llave secreta inválida", errHumanCheckUnavailable)
			}
		}
		return errNotHuman
	}
	return nil
}

// clientIP es la IP de quien hace la solicitud: la primera de X-Forwarded-For (Cloud
// Run la pone) o, si no viene, la de la conexión.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

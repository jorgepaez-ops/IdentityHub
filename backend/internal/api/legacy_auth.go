// ╔══════════════════════════════════════════════════════════════════════════╗
// ║  ⚠️  ARCHIVO DELIBERADAMENTE VULNERABLE — NO CORREGIR SIN LEER ESTO       ║
// ║                                                                          ║
// ║  Ver specs/adr/0007-linea-base-vulnerable-deliberada.md                   ║
// ║                                                                          ║
// ║  Cada defecto de este archivo está sembrado a propósito para que un gate  ║
// ║  concreto del pipeline tenga algo real que informar. Sin esto, un job en  ║
// ║  verde es indistinguible de un job mal configurado.                       ║
// ║                                                                          ║
// ║    VULN-001  credenciales incrustadas         → Gitleaks, gosec G101      ║
// ║    VULN-002  MD5 para contraseñas             → gosec G401/G501, CodeQL   ║
// ║    VULN-004  math/rand para tokens            → gosec G404                ║
// ║    VULN-005  SQL por concatenación            → gosec G201, CodeQL, Semgrep║
// ║    VULN-006  JWT sin validar el algoritmo     → Semgrep, CodeQL           ║
// ║    VULN-007  CORS comodín con credenciales    → Semgrep, ZAP              ║
// ║                                                                          ║
// ║  Este endpoint NO figura en specs/03-api/openapi.yaml: no forma parte del ║
// ║  contrato. Se elimina entero en la fase de remediación (semana 3) y su    ║
// ║  desaparición es parte de la evidencia.                                   ║
// ║                                                                          ║
// ║  Las credenciales de abajo son inventadas y no dan acceso a nada.         ║
// ╚══════════════════════════════════════════════════════════════════════════╝

package api

import (
	"crypto/md5" //nolint:gosec // VULN-002: sembrado a propósito
	"encoding/hex"
	"fmt"
	"math/rand" //nolint:gosec // VULN-004: sembrado a propósito
	"net/http"

	"github.com/golang-jwt/jwt/v4"
)

// VULN-001 — Credenciales incrustadas en el código fuente.
// Gitleaks las encuentra recorriendo el historial completo; gosec las marca
// como G101. En el código remediado estos valores vienen de config.Load() y su
// ausencia impide arrancar.
const (
	legacySigningKey = "clave-de-firma-super-secreta-2024"
	legacyAdminUser  = "admin"
	legacyAdminPass  = "Admin123!"
	legacyAWSKey     = "AKIAIOSFODNN7EXAMPLE"
	legacyAWSSecret  = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	legacySlackToken = "xoxb-1234567890-0987654321-AbCdEfGhIjKlMnOpQrStUvWx"
)

// VULN-002 — MD5 como función de derivación de contraseñas.
// MD5 está roto para colisiones y, sobre todo, es rapidísimo: una GPU prueba
// miles de millones de candidatos por segundo. Lo correcto es Argon2id con
// coste en memoria (ver specs/adr/0004).
func legacyHashPassword(password string) string {
	sum := md5.Sum([]byte(password)) //nolint:gosec // VULN-002
	return hex.EncodeToString(sum[:])
}

// VULN-004 — math/rand para material criptográfico.
// El generador de math/rand es predecible: quien observe unas cuantas salidas
// puede reconstruir el estado interno y predecir los tokens siguientes.
// Lo correcto es crypto/rand.
func legacyGenerateToken() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, 32)
	for i := range out {
		out[i] = alphabet[rand.Intn(len(alphabet))] //nolint:gosec // VULN-004
	}
	return string(out)
}

// VULN-005 — Consulta construida por concatenación de cadenas.
// Una entrada como `' OR '1'='1` convierte la condición en universalmente
// verdadera. Lo correcto son parámetros vinculados ($1), que es lo que genera
// sqlc en el código remediado.
func legacyBuildUserQuery(email string) string {
	return fmt.Sprintf("SELECT id, email, password_hash FROM users WHERE email = '%s'", email)
}

// VULN-006 — Validación de JWT sin comprobar el algoritmo de firma.
// Al no verificar el método, se acepta un token con `alg: none` o uno firmado
// con HMAC usando la clave pública como secreto. Es la amenaza AM-003 del
// modelo de amenazas, materializada.
func legacyParseToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(_ *jwt.Token) (any, error) {
		return []byte(legacySigningKey), nil
	})
}

// LegacyLogin reúne todo lo anterior en un handler alcanzable, requisito para
// que govulncheck informe del CVE de golang-jwt v4: esa herramienta solo
// reporta vulnerabilidades cuyo código es realmente invocable.
func (s *Server) LegacyLogin(w http.ResponseWriter, r *http.Request) {
	// VULN-007 — CORS comodín junto con credenciales. Los navegadores rechazan
	// esta combinación precisamente porque anula la política del mismo origen.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Si llega un token, se "valida" con la función rota de arriba. Esto hace
	// que jwt.Parse sea alcanzable, requisito para que govulncheck informe del
	// CVE de golang-jwt v4: la herramienta solo reporta código realmente
	// invocable, no la mera presencia de la dependencia en go.mod.
	if raw := r.Header.Get("Authorization"); raw != "" {
		if tok, err := legacyParseToken(raw); err == nil && tok.Valid {
			writeJSON(w, http.StatusOK, map[string]any{"claims": tok.Claims})
			return
		}
	}

	email := r.URL.Query().Get("email")
	password := r.URL.Query().Get("password")

	query := legacyBuildUserQuery(email)
	s.logger.Info("consulta legacy construida", "query", query)

	if email == legacyAdminUser && legacyHashPassword(password) == legacyHashPassword(legacyAdminPass) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":  email,
			"sid":  legacyGenerateToken(),
			"role": "admin",
		})
		signed, err := token.SignedString([]byte(legacySigningKey))
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "internal", "Error interno", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": signed})
		return
	}

	writeProblem(w, http.StatusUnauthorized, "invalid-credentials", "Credenciales inválidas", "")
}

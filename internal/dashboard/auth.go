// Package dashboard implementa un panel de administración TUI (estilo Filament)
// pensado para ejecutarse dentro de una sesión SSH ya autenticada. No abre
// puertos ni expone servicios: reutiliza la persistencia y la configuración del
// proyecto para editar clientes, tareas, mensajes, .env y ver actividad.
package dashboard

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 120_000
	pbkdf2KeyLen     = 32
	pbkdf2Prefix     = "pbkdf2_sha256"
)

// HashPassword genera un hash PBKDF2-SHA256 con formato
// pbkdf2_sha256$<iteraciones>$<salt>$<hash>.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s",
		pbkdf2Prefix,
		pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword compara una contraseña con un hash almacenado en tiempo constante.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Prefix {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Auth valida el acceso de un único usuario. Prefiere el hash PBKDF2; si no hay
// hash configurado acepta una contraseña en claro (útil en desarrollo).
type Auth struct {
	User     string
	hash     string
	password string
}

// NewAuth construye el validador de acceso.
func NewAuth(user, hash, password string) *Auth {
	if strings.TrimSpace(user) == "" {
		user = "admin"
	}
	return &Auth{User: user, hash: strings.TrimSpace(hash), password: password}
}

// Configured indica si hay credenciales definidas.
func (a *Auth) Configured() bool { return a.hash != "" || a.password != "" }

// Verify comprueba usuario y contraseña.
func (a *Auth) Verify(user, password string) bool {
	if subtle.ConstantTimeCompare([]byte(user), []byte(a.User)) != 1 {
		return false
	}
	if a.hash != "" {
		return VerifyPassword(a.hash, password)
	}
	if a.password != "" {
		return subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
	}
	return password == ""
}

// Command gmail-auth ejecuta el flujo OAuth 2.0 de Gmail una sola vez y guarda
// el refresh token en .env, para que core-engine renueve el access token solo.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	authEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint = "https://oauth2.googleapis.com/token"
	defaultScope  = "https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/pubsub"
)

func main() {
	clientID := flag.String("client-id", "", "OAuth client_id (o GMAIL_CLIENT_ID en .env)")
	clientSecret := flag.String("client-secret", "", "OAuth client_secret (o GMAIL_CLIENT_SECRET en .env)")
	scope := flag.String("scope", defaultScope, "scope OAuth a solicitar")
	envPath := flag.String("env", ".env", "archivo .env a actualizar")
	noBrowser := flag.Bool("no-browser", false, "no intentar abrir el navegador")
	flag.Parse()

	dotenv := loadDotEnv(*envPath)

	if *clientID == "" {
		*clientID = firstNonEmpty(os.Getenv("GMAIL_CLIENT_ID"), dotenv["GMAIL_CLIENT_ID"])
	}
	if *clientSecret == "" {
		*clientSecret = firstNonEmpty(os.Getenv("GMAIL_CLIENT_SECRET"), dotenv["GMAIL_CLIENT_SECRET"])
	}
	if *clientID == "" || *clientSecret == "" {
		fatal("faltan client_id/client_secret: pasalos con -client-id/-client-secret o cargalos en " + *envPath)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal("no se pudo abrir el puerto local: " + err.Error())
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d", port)
	state := fmt.Sprintf("st-%d", time.Now().UnixNano())

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "Autorización rechazada: "+e, http.StatusBadRequest)
			errCh <- fmt.Errorf("autorización rechazada: %s", e)
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state inválido", http.StatusBadRequest)
			errCh <- errors.New("state inválido")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "sin code", http.StatusBadRequest)
			errCh <- errors.New("respuesta sin code")
			return
		}
		fmt.Fprint(w, "<html><body><h2>Listo</h2><p>Ya podés cerrar esta pestaña.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	authURL := buildAuthURL(*clientID, redirectURI, *scope, state)
	fmt.Println("Abrí esta URL en el navegador para autorizar Gmail:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	if !*noBrowser {
		openBrowser(authURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		fatal(err.Error())
	case <-ctx.Done():
		fatal("timeout esperando la autorización")
	}

	tokens, err := exchange(ctx, *clientID, *clientSecret, code, redirectURI)
	if err != nil {
		fatal(err.Error())
	}
	if tokens.RefreshToken == "" {
		fatal("Google no devolvió refresh_token; revocá el acceso previo en https://myaccount.google.com/permissions y reintentá")
	}

	if err := updateDotEnv(*envPath, map[string]string{
		"GMAIL_CLIENT_ID":     *clientID,
		"GMAIL_CLIENT_SECRET": *clientSecret,
		"GMAIL_REFRESH_TOKEN": tokens.RefreshToken,
	}); err != nil {
		fatal("no se pudo actualizar " + *envPath + ": " + err.Error())
	}

	fmt.Println()
	fmt.Println("Refresh token obtenido y guardado en", *envPath)
	fmt.Println("GMAIL_REFRESH_TOKEN=" + tokens.RefreshToken)
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func exchange(ctx context.Context, clientID, clientSecret, code, redirectURI string) (tokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return tokenResponse{}, err
	}

	var body tokenResponse
	if err := json.Unmarshal(data, &body); err != nil {
		return tokenResponse{}, err
	}
	if resp.StatusCode != http.StatusOK || body.AccessToken == "" {
		msg := firstNonEmpty(body.ErrorDescription, body.Error, strings.TrimSpace(string(data)))
		return tokenResponse{}, fmt.Errorf("intercambio de código status %d: %s", resp.StatusCode, msg)
	}
	return body, nil
}

func buildAuthURL(clientID, redirectURI, scope, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", scope)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("state", state)
	return authEndpoint + "?" + q.Encode()
}

func openBrowser(u string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = append(args, "url.dll,FileProtocolHandler")
	default:
		cmd = "xdg-open"
	}
	args = append(args, u)
	_ = exec.Command(cmd, args...).Start()
}

func loadDotEnv(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

func updateDotEnv(path string, updates map[string]string) error {
	var lines []string
	data, err := os.ReadFile(path)
	if err == nil {
		lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	} else if !os.IsNotExist(err) {
		return err
	}

	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if v, ok := updates[k]; ok {
			lines[i] = k + "=" + v
			seen[k] = true
		}
	}
	for k, v := range updates {
		if !seen[k] {
			lines = append(lines, k+"="+v)
		}
	}

	out := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(out), 0o600)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

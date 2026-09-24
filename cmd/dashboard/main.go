// Command dashboard es un panel de administración TUI (estilo Filament) para
// K-OS. Está pensado para ejecutarse dentro de una sesión SSH ya autenticada:
// no abre puertos ni expone servicios. Permite gestionar clientes, tareas,
// mensajes, editar el .env y ver la actividad del panel.
//
// Uso:
//
//	go run ./cmd/dashboard                 # usa .env y DATABASE_DSN del entorno
//	go run ./cmd/dashboard -env .env       # ruta explícita del .env
//	go run ./cmd/dashboard -hash           # genera el hash de la contraseña
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/dashboard"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

func main() {
	envPath := flag.String("env", ".env", "ruta del archivo .env editable")
	dsnFlag := flag.String("dsn", "", "DSN de MariaDB (por defecto DATABASE_DSN o construido desde .env)")
	userFlag := flag.String("user", "", "usuario del panel (por defecto DASHBOARD_USER o admin)")
	dbHost := flag.String("db-host", "127.0.0.1", "host de MariaDB al construir el DSN desde .env")
	dbPort := flag.String("db-port", "3307", "puerto de MariaDB al construir el DSN desde .env")
	genHash := flag.Bool("hash", false, "pide una contraseña y muestra su hash PBKDF2, luego sale")
	flag.Parse()

	if *genHash {
		if err := printHash(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	env, err := dashboard.LoadEnvFile(*envPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo leer el .env:", err)
		os.Exit(1)
	}

	authUser := firstNonEmpty(*userFlag, os.Getenv("DASHBOARD_USER"), envGet(env, "DASHBOARD_USER"), "admin")
	authHash := firstNonEmpty(os.Getenv("DASHBOARD_PASSWORD_HASH"), envGet(env, "DASHBOARD_PASSWORD_HASH"))
	authPass := firstNonEmpty(os.Getenv("DASHBOARD_PASSWORD"), envGet(env, "DASHBOARD_PASSWORD"))
	auth := dashboard.NewAuth(authUser, authHash, authPass)

	dsn := firstNonEmpty(*dsnFlag, os.Getenv("DATABASE_DSN"), buildDSN(env, *dbHost, *dbPort))

	ctx := context.Background()
	var store persistence.Store
	backend := "memoria (demo)"
	if dsn != "" {
		ms, err := persistence.NewMySQLStore(ctx, dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "no se pudo conectar a MariaDB:", err)
			os.Exit(1)
		}
		defer ms.Close()
		store = ms
		backend = "MariaDB"
	} else {
		store = persistence.NewMemoryStore()
	}

	d := dashboard.New(dashboard.Options{
		Store:   store,
		EnvPath: *envPath,
		Auth:    auth,
		Backend: backend,
	})

	if err := d.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "dashboard:", err)
		os.Exit(1)
	}
}

func printHash() error {
	var result string
	field := tview.NewInputField().SetLabel("Contraseña ").SetMaskCharacter('*').SetFieldWidth(30)
	form := tview.NewForm().
		AddFormItem(field).
		AddButton("Generar", func() {
			h, err := dashboard.HashPassword(field.GetText())
			if err != nil {
				return
			}
			result = h
		}).
		AddButton("Salir", func() {})
	form.SetBorder(true).SetTitle(" Generar hash de contraseña ").SetTitleAlign(tview.AlignCenter)

	app := tview.NewApplication()
	app.SetRoot(form, true)
	if err := app.Run(); err != nil {
		return err
	}
	if result != "" {
		fmt.Println(result)
	}
	return nil
}

func envGet(env *dashboard.EnvFile, key string) string {
	v, _ := env.Get(key)
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func buildDSN(env *dashboard.EnvFile, host, port string) string {
	user := envGet(env, "MARIADB_USER")
	pass := envGet(env, "MARIADB_PASSWORD")
	name := envGet(env, "MARIADB_DATABASE")
	if user == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true", user, pass, host, port, name)
}

package dashboard

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/persistence"
)

// view es una sección del dashboard.
type view interface {
	Primitive() tview.Primitive
	Refresh()
}

// Options configura el dashboard.
type Options struct {
	Store   persistence.Store
	EnvPath string
	Auth    *Auth
	Backend string
	Logger  *slog.Logger
}

// Dashboard es el panel TUI.
type Dashboard struct {
	app      *tview.Application
	pages    *tview.Pages
	store    persistence.Store
	envPath  string
	env      *EnvFile
	auth     *Auth
	backend  string
	activity *ActivityLog

	user    string
	current string
	content *tview.Pages
	menu    *tview.List
	header  *tview.TextView
	footer  *tview.TextView
	views   map[string]view
	done    chan struct{}
}

// New crea el dashboard.
func New(opts Options) *Dashboard {
	d := &Dashboard{
		store:    opts.Store,
		envPath:  opts.EnvPath,
		auth:     opts.Auth,
		backend:  opts.Backend,
		activity: NewActivityLog(500),
		done:     make(chan struct{}),
	}
	if d.auth == nil {
		d.auth = NewAuth("admin", "", "")
	}
	env, err := LoadEnvFile(opts.EnvPath)
	if err != nil {
		d.activity.Add("no se pudo leer %s: %v", opts.EnvPath, err)
		env = &EnvFile{path: opts.EnvPath}
	}
	d.env = env
	if opts.Logger != nil {
		opts.Logger.Info("dashboard listo", "env", opts.EnvPath, "backend", opts.Backend)
	}
	return d
}

// Run arranca el dashboard (bloquea hasta salir).
func (d *Dashboard) Run() error {
	d.app = tview.NewApplication()
	d.app.EnableMouse(true)
	d.app.SetInputCapture(d.capture)
	d.pages = tview.NewPages()
	applyTheme()
	d.buildLogin()
	d.pages.SwitchToPage("login")
	d.app.SetRoot(d.pages, true)
	defer close(d.done)
	return d.app.Run()
}

func (d *Dashboard) capture(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEscape && d.pages.HasPage("modal") {
		d.closeModal()
		return nil
	}
	if ev.Key() == tcell.KeyTab && d.content != nil && !d.pages.HasPage("modal") {
		d.togglePanel()
		return nil
	}
	return ev
}

func (d *Dashboard) togglePanel() {
	if d.app.GetFocus() == d.menu {
		if v, ok := d.views[d.current]; ok {
			d.app.SetFocus(v.Primitive())
		}
	} else {
		d.app.SetFocus(d.menu)
	}
}

// ── Login ────────────────────────────────────────────────────────

func (d *Dashboard) buildLogin() {
	userField := tview.NewInputField().SetLabel("Usuario     ").SetText(d.auth.User).SetFieldWidth(24)
	passField := tview.NewInputField().SetLabel("Contraseña  ").SetFieldWidth(24).SetMaskCharacter('*')
	status := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	submit := func() {
		u := strings.TrimSpace(userField.GetText())
		p := passField.GetText()
		if d.auth.Verify(u, p) {
			d.user = u
			d.activity.Add("acceso correcto de %q", u)
			d.buildShell()
			d.pages.SwitchToPage("shell")
			d.navigate("home")
			return
		}
		d.activity.Add("acceso fallido para %q", u)
		status.SetText("[red]Credenciales inválidas[-]")
		passField.SetText("")
		d.app.SetFocus(passField)
	}

	form := tview.NewForm().
		AddFormItem(userField).
		AddFormItem(passField).
		AddButton("Entrar", submit).
		AddButton("Salir", func() { d.app.Stop() })
	form.SetBorder(true).SetTitle(" K-OS · Acceso al panel ").SetTitleAlign(tview.AlignCenter)

	if !d.auth.Configured() {
		status.SetText("[yellow]Sin DASHBOARD_PASSWORD_HASH configurado: acceso abierto (solo desarrollo)[-]")
	}

	centered := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(form, 52, 1, true).
			AddItem(nil, 0, 1, false), 11, 1, true).
		AddItem(status, 2, 0, false).
		AddItem(nil, 0, 1, false)

	d.pages.AddPage("login", centered, true, true)
	d.app.SetFocus(userField)
}

// ── Shell ────────────────────────────────────────────────────────

func (d *Dashboard) buildShell() {
	d.content = tview.NewPages()
	d.views = map[string]view{
		"home":     newHomeView(d),
		"tasks":    newTasksView(d),
		"clients":  newClientsView(d),
		"messages": newMessagesView(d),
		"config":   newConfigView(d),
		"logs":     newLogsView(d),
	}
	for _, name := range []string{"home", "tasks", "clients", "messages", "config", "logs"} {
		d.content.AddPage(name, d.views[name].Primitive(), true, name == "home")
	}

	d.menu = tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	d.menu.SetBorder(true).SetTitle(" Menú ")
	d.menu.AddItem(" Inicio", "", 'i', func() { d.navigate("home") })
	d.menu.AddItem(" Tareas", "", 't', func() { d.navigate("tasks") })
	d.menu.AddItem(" Clientes", "", 'c', func() { d.navigate("clients") })
	d.menu.AddItem(" Mensajes", "", 'm', func() { d.navigate("messages") })
	d.menu.AddItem(" Configuración", "", 'o', func() { d.navigate("config") })
	d.menu.AddItem(" Logs", "", 'l', func() { d.navigate("logs") })
	d.menu.AddItem(" Salir", "", 'q', func() { d.app.Stop() })

	d.header = tview.NewTextView().SetDynamicColors(true)
	d.footer = tview.NewTextView().SetDynamicColors(true)
	d.footer.SetText(" [gray]Tab cambiar panel · ↑↓/Enter mover/abrir · Ratón activo · Esc cerrar modal · Ctrl+C salir[-]")

	main := tview.NewFlex().
		AddItem(d.menu, 24, 1, true).
		AddItem(d.content, 0, 4, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.header, 1, 0, false).
		AddItem(main, 0, 1, true).
		AddItem(d.footer, 1, 0, false)

	d.pages.AddPage("shell", layout, true, true)
	d.app.SetFocus(d.menu)
	d.setHeader("home")
}

var sectionTitles = map[string]string{
	"home":     "Inicio",
	"tasks":    "Tareas",
	"clients":  "Clientes",
	"messages": "Mensajes",
	"config":   "Configuración (.env)",
	"logs":     "Logs",
}

func (d *Dashboard) navigate(name string) {
	d.current = name
	d.content.SwitchToPage(name)
	if v, ok := d.views[name]; ok {
		v.Refresh()
		d.app.SetFocus(v.Primitive())
	}
	d.setHeader(name)
}

func (d *Dashboard) setHeader(section string) {
	title := sectionTitles[section]
	if title == "" {
		title = section
	}
	d.header.SetText(" [green::b]K-OS[-:-:-] · " + title +
		"   [gray]usuario:[-] " + d.user +
		"   [gray]backend:[-] " + d.backend +
		"   [gray]" + time.Now().Format("2006-01-02 15:04") + "[-]")
}

// ── Modales ──────────────────────────────────────────────────────

func (d *Dashboard) openModal(content tview.Primitive, width, height int) {
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(content, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	d.pages.RemovePage("modal")
	d.pages.AddPage("modal", modal, true, true)
	d.pages.ShowPage("modal")
	d.app.SetFocus(content)
}

func (d *Dashboard) closeModal() {
	d.pages.RemovePage("modal")
	if d.content != nil {
		d.app.SetFocus(d.content)
	}
}

func (d *Dashboard) confirm(text string, onYes func()) {
	m := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Cancelar", "Confirmar"}).
		SetDoneFunc(func(_ int, label string) {
			d.closeModal()
			if label == "Confirmar" {
				onYes()
			}
		})
	d.openModal(m, 62, 9)
}

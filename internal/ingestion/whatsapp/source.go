// Package whatsapp implementa la ingesta de mensajes de WhatsApp mediante
// whatsmeow (WhatsApp Web multi-dispositivo). La sesión se persiste en SQLite;
// los mensajes autorizados se normalizan a IncomingMessage y entran en el mismo
// pipeline que Gmail.
package whatsapp

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/nestorPons/ai-assistant/internal/domain"

	_ "modernc.org/sqlite"
)

// Config ajusta el listener de WhatsApp.
type Config struct {
	DBPath string
	Logger *slog.Logger
}

// Source escucha WhatsApp y emite IncomingMessage al canal de salida.
type Source struct {
	dbPath string
	logger *slog.Logger
	log    waLog.Logger

	ctx       context.Context
	container *sqlstore.Container
	client    *whatsmeow.Client

	mu     sync.RWMutex
	status Status
}

// New crea una fuente WhatsApp.
func New(cfg Config) *Source {
	if cfg.DBPath == "" {
		cfg.DBPath = "whatsapp.db"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Source{
		dbPath: cfg.DBPath,
		logger: cfg.Logger,
		log:    newSlogLogger(cfg.Logger, "whatsapp"),
	}
}

// Name identifica el canal.
func (s *Source) Name() string { return "whatsapp" }

// Status devuelve el estado actual del canal (conectado / QR pendiente).
func (s *Source) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Source) setStatus(st Status) {
	s.mu.Lock()
	s.status = st
	s.mu.Unlock()
}

// Start conecta el cliente y bloquea hasta que el contexto se cancela. Si no
// hay sesión guardada, emite el QR y espera el emparejado (reintentando).
func (s *Source) Start(ctx context.Context, out chan<- domain.IncomingMessage) error {
	s.ctx = ctx

	client, err := s.newClient(ctx)
	if err != nil {
		return err
	}
	s.client = client
	defer func() {
		if s.container != nil {
			_ = s.container.Close()
		}
	}()

	client.AddEventHandler(func(evt any) { s.handleEvent(client, evt, out) })

	if client.Store.ID != nil {
		s.setStatus(Status{Connected: client.IsLoggedIn()})
		if err := client.Connect(); err != nil && ctx.Err() == nil {
			s.logger.Error("whatsapp: conexión fallida", "error", err)
		}
		<-ctx.Done()
		client.Disconnect()
		return ctx.Err()
	}

	// Sin sesión: bucle de emparejado por QR hasta loguear o cancelar.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.pair(ctx, client)
		if client.IsLoggedIn() {
			s.setStatus(Status{Connected: true})
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	<-ctx.Done()
	client.Disconnect()
	return ctx.Err()
}

func (s *Source) newClient(ctx context.Context) (*whatsmeow.Client, error) {
	db, err := sql.Open("sqlite", s.dsn())
	if err != nil {
		return nil, err
	}
	// SQLite: una única conexión evita "database is locked".
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	container := sqlstore.NewWithDB(db, "sqlite3", s.log)
	if err := container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	device.Log = s.log

	s.container = container
	return whatsmeow.NewClient(device, s.log), nil
}

func (s *Source) dsn() string {
	abs, err := filepath.Abs(s.dbPath)
	if err != nil {
		abs = s.dbPath
	}
	if dir := filepath.Dir(abs); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	return "file:" + abs + "?_busy_timeout=10000&_journal_mode=wal&_foreign_keys=on"
}

// pair inicia el emparejado por QR y consume el canal hasta que se cierra.
func (s *Source) pair(ctx context.Context, client *whatsmeow.Client) {
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		s.setStatus(Status{Error: err.Error()})
		s.logger.Error("whatsapp: no se pudo iniciar el emparejado", "error", err)
		return
	}
	if err := client.Connect(); err != nil {
		s.setStatus(Status{Error: err.Error()})
		s.logger.Error("whatsapp: error al conectar para emparejar", "error", err)
		return
	}
	s.logger.Info("whatsapp: escanea el código QR en WhatsApp (Ajustes > Dispositivos vinculados)")

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-qrChan:
			if !ok {
				return
			}
			switch evt.Event {
			case whatsmeow.QRChannelEventCode:
				s.setStatus(Status{QR: evt.Code})
			case whatsmeow.QRChannelSuccess.Event:
				s.setStatus(Status{Connected: true})
				s.logger.Info("whatsapp: emparejado correctamente")
			case whatsmeow.QRChannelEventError:
				msg := errorText(evt.Error)
				s.setStatus(Status{Error: msg})
				s.logger.Error("whatsapp: error de emparejado", "error", msg)
			default:
				s.setStatus(Status{Error: evt.Event})
				s.logger.Warn("whatsapp: emparejado finalizado", "event", evt.Event)
			}
		}
	}
}

func (s *Source) handleEvent(client *whatsmeow.Client, raw any, out chan<- domain.IncomingMessage) {
	switch evt := raw.(type) {
	case *events.Message:
		s.handleMessage(evt, out)
	case *events.PairSuccess:
		s.setStatus(Status{Connected: true})
	case *events.Connected:
		s.setStatus(Status{Connected: client.IsLoggedIn()})
	case *events.Disconnected:
		if client.IsLoggedIn() {
			s.setStatus(Status{Connected: false, Error: "desconectado, reintentando"})
		}
	case *events.LoggedOut:
		s.setStatus(Status{Connected: false, Error: "sesión de WhatsApp cerrada; reinicia el servicio para volver a emparejar"})
	}
}

func (s *Source) handleMessage(evt *events.Message, out chan<- domain.IncomingMessage) {
	if evt.Info.IsFromMe {
		return
	}
	// Solo chats 1:1 y grupos; se ignoran estados/difusiones.
	if evt.Info.Chat.Server != types.DefaultUserServer && evt.Info.Chat.Server != types.GroupServer {
		return
	}

	text := messageText(evt.Message)
	if text == "" {
		return
	}

	identifier := clientIdentifier(evt.Info)
	if identifier == "" {
		return
	}

	msg := domain.IncomingMessage{
		ID:               string(evt.Info.ID),
		Source:           domain.SourceWhatsApp,
		ClientIdentifier: identifier,
		ClientName:       evt.Info.PushName,
		RawContent:       text,
		ReceivedAt:       evt.Info.Timestamp.UTC(),
	}

	select {
	case <-s.ctx.Done():
	case out <- msg:
	}
}

// clientIdentifier devuelve el identificador autorizable: teléfono (+34…)
// para chats 1:1 y el JID completo del grupo (<id>@g.us) para grupos.
func clientIdentifier(info types.MessageInfo) string {
	if info.IsGroup {
		return info.Chat.String()
	}
	if info.Sender.User == "" {
		return ""
	}
	return "+" + info.Sender.User
}

// messageText extrae el texto del mensaje: conversación, texto extendido o el
// pie de foto de los medios. Devuelve "" si no hay texto relevante.
func messageText(m *waE2E.Message) string {
	if m == nil {
		return ""
	}
	if c := m.GetConversation(); c != "" {
		return c
	}
	if et := m.GetExtendedTextMessage(); et != nil {
		return et.GetText()
	}
	if img := m.GetImageMessage(); img != nil {
		return img.GetCaption()
	}
	if vid := m.GetVideoMessage(); vid != nil {
		return vid.GetCaption()
	}
	if doc := m.GetDocumentMessage(); doc != nil {
		return doc.GetCaption()
	}
	return ""
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

package whatsapp

import (
	"errors"

	"github.com/skip2/go-qrcode"
)

// Status describe el estado público del canal WhatsApp para exponerlo en el
// panel: conectado o el QR pendiente de escanear.
type Status struct {
	Connected bool   `json:"connected"`
	QR        string `json:"qr,omitempty"`
	Error     string `json:"error,omitempty"`
}

// QRPNG renderiza el payload del QR pendiente como imagen PNG.
func (s *Source) QRPNG() ([]byte, error) {
	payload := s.statusPayload()
	if payload == "" {
		return nil, errors.New("sin código QR disponible")
	}
	return qrcode.Encode(payload, qrcode.Medium, 256)
}

// WhatsAppStatus implementa httpapi.WhatsAppStatusProvider.
func (s *Source) WhatsAppStatus() (connected bool, qr, errMsg string) {
	st := s.Status()
	return st.Connected, st.QR, st.Error
}

// WhatsAppQRPNG implementa httpapi.WhatsAppStatusProvider.
func (s *Source) WhatsAppQRPNG() ([]byte, error) {
	return s.QRPNG()
}

func (s *Source) statusPayload() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status.QR
}

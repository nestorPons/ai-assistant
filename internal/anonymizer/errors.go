package anonymizer

import "errors"

var (
	// ErrMappingNotFound indica un request_id inexistente o expirado.
	ErrMappingNotFound = errors.New("mapping no encontrado")
)

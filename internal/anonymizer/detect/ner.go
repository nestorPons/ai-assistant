package detect

// NER es la interfaz de detección contextual (opcional). En el MVP se usa una
// implementación vacía; puede sustituirse por inferencia ONNX sin cambiar la API.
type NER interface {
	Detect(text string) []Detection
}

// NoopNER no detecta entidades contextuales.
type NoopNER struct{}

// Detect devuelve sin detecciones.
func (NoopNER) Detect(string) []Detection { return nil }

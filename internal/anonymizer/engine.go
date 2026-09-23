package anonymizer

import (
	"sort"
	"strings"

	"github.com/nestorPons/ai-assistant/internal/anonymizer/detect"
)

// Engine combina detección heurística y contextual, resuelve solapamientos y
// sustituye PII por marcadores estables. El valor original se conserva solo en
// la MappingTable.
type Engine struct {
	detector *detect.Detector
	ner      detect.NER
}

// NewEngine crea un motor de anonimización.
func NewEngine(d *detect.Detector, ner detect.NER) *Engine {
	if ner == nil {
		ner = detect.NoopNER{}
	}
	return &Engine{detector: d, ner: ner}
}

// Sanitize ofusca el texto y devuelve cleanPrompt, entidades y la tabla de tokens.
func (e *Engine) Sanitize(text string) (string, []EntityMatch, MappingTable) {
	detections := e.detector.Detect(text)
	detections = append(detections, e.ner.Detect(text)...)
	detections = resolveOverlaps(detections)

	mapping := MappingTable{}
	entities := make([]EntityMatch, 0, len(detections))
	type subst struct {
		start, end int
		token      string
	}
	substs := make([]subst, 0, len(detections))

	// índice por tipo, y por tipo+valor para estabilidad de marcadores.
	typeCounter := map[detect.EntityType]int{}
	valueIndex := map[detect.EntityType]map[string]int{}

	for _, det := range detections {
		value := text[det.Start:det.End]

		if valueIndex[det.Type] == nil {
			valueIndex[det.Type] = map[string]int{}
		}
		norm := strings.ToLower(strings.TrimSpace(value))

		idx, ok := valueIndex[det.Type][norm]
		if !ok {
			typeCounter[det.Type]++
			idx = typeCounter[det.Type]
			valueIndex[det.Type][norm] = idx
		}

		token := "{{" + det.Type.Prefix() + "_" + itoa(idx) + "}}"
		mapping[token] = value
		entities = append(entities, EntityMatch{
			Type:  det.Type,
			Token: token,
			Start: det.Start,
			End:   det.End,
		})
		substs = append(substs, subst{det.Start, det.End, token})
	}

	// Sustituir de derecha a izquierda para no desplazar los índices.
	repl := text
	for i := len(substs) - 1; i >= 0; i-- {
		s := substs[i]
		repl = repl[:s.start] + s.token + repl[s.end:]
	}

	return repl, entities, mapping
}

// resolveOverlaps ordena, deduplica y elimina solapamientos, priorizando las
// coincidencias más específicas (más largas) y evitando reemplazar dos veces.
func resolveOverlaps(dets []detect.Detection) []detect.Detection {
	if len(dets) == 0 {
		return nil
	}
	sort.SliceStable(dets, func(i, j int) bool {
		if dets[i].Start != dets[j].Start {
			return dets[i].Start < dets[j].Start
		}
		// más específica primero
		if dets[i].End != dets[j].End {
			return dets[i].End > dets[j].End
		}
		return dets[i].Type < dets[j].Type
	})

	out := make([]detect.Detection, 0, len(dets))
	lastEnd := -1
	for _, d := range dets {
		if d.Start < lastEnd {
			continue
		}
		if d.End <= d.Start {
			continue
		}
		out = append(out, d)
		lastEnd = d.End
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

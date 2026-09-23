// Package detect implementa la detección heurística de PII: regex, validadores
// exactos (DNI/NIE, IBAN, Luhn) y búsqueda por diccionarios en memoria.
package detect

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	ahocorasick "github.com/BobuSumisu/aho-corasick"
)

// EntityType identifica la clase de entidad detectada.
type EntityType string

const (
	EntityPerson EntityType = "PERSON"
	EntityOrg    EntityType = "ORG"
	EntityLoc    EntityType = "LOC"
	EntityEmail  EntityType = "EMAIL"
	EntityPhone  EntityType = "PHONE"
	EntityDNI    EntityType = "DNI"
	EntityIBAN   EntityType = "IBAN"
	EntityCard   EntityType = "CARD"
)

// tokenPrefix asocia cada tipo con el prefijo del marcador estable.
var tokenPrefix = map[EntityType]string{
	EntityPerson: "PERSONA",
	EntityOrg:    "ORGANIZACION",
	EntityLoc:    "UBICACION",
	EntityEmail:  "EMAIL",
	EntityPhone:  "TELEFONO",
	EntityDNI:    "DNI",
	EntityIBAN:   "IBAN",
	EntityCard:   "TARJETA",
}

// Prefix devuelve el prefijo del marcador para un tipo.
func (t EntityType) Prefix() string {
	if p, ok := tokenPrefix[t]; ok {
		return p
	}
	return "ENTITY"
}

// Detection describe una entidad detectada con su rango en el texto original.
type Detection struct {
	Type  EntityType
	Start int
	End   int
}

// Detector combina los detectores heurísticos.
type Detector struct {
	emailRe  *regexp.Regexp
	phoneRe  *regexp.Regexp
	dniRe    *regexp.Regexp
	ibanRe   *regexp.Regexp
	cardRe   *regexp.Regexp
	trie     *ahocorasick.Trie
	dictType map[string]EntityType
}

// New crea un Detector con los diccionarios de personas, organizaciones y lugares.
func New(personDict, orgDict, locDict []string) *Detector {
	d := &Detector{
		emailRe:  regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`),
		phoneRe:  regexp.MustCompile(`(?:\+34|0034|34)?[ -]?[6789]\d{2}[ -]?\d{3}[ -]?\d{3}\b`),
		dniRe:    regexp.MustCompile(`(?i)\b[0-9XYZ][0-9]{7}[TRWAGMYFPDXBNJZSQVHLCKE]\b`),
		ibanRe:   regexp.MustCompile(`(?i)\b[A-Z]{2}[0-9]{2}[ -]?[0-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{0,4}\b`),
		cardRe:   regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`),
		dictType: make(map[string]EntityType),
	}

	builder := ahocorasick.NewTrieBuilder()
	addDict := func(entries []string, t EntityType) {
		for _, e := range entries {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			lower := strings.ToLower(e)
			builder.AddString(lower)
			d.dictType[lower] = t
		}
	}
	addDict(personDict, EntityPerson)
	addDict(orgDict, EntityOrg)
	addDict(locDict, EntityLoc)

	d.trie = builder.Build()
	return d
}

// Detect ejecuta la fase rápida y devuelve las entidades detectadas.
func (d *Detector) Detect(text string) []Detection {
	var out []Detection

	out = append(out, d.matchRegex(d.emailRe, text, EntityEmail)...)
	out = append(out, d.matchRegex(d.phoneRe, text, EntityPhone)...)
	out = append(out, d.matchRegex(d.dniRe, text, EntityDNI, isDniNIE)...)
	out = append(out, d.matchRegex(d.ibanRe, text, EntityIBAN, isIBAN)...)
	out = append(out, d.matchRegex(d.cardRe, text, EntityCard, isCard)...)
	out = append(out, d.matchDictionary(text)...)

	return out
}

// matchRegex aplica una regex y opcionalmente un validador exacto.
func (d *Detector) matchRegex(re *regexp.Regexp, text string, t EntityType, validators ...func(string) bool) []Detection {
	var out []Detection
	for _, loc := range re.FindAllStringIndex(text, -1) {
		s := text[loc[0]:loc[1]]
		for _, v := range validators {
			if !v(s) {
				goto next
			}
		}
		out = append(out, Detection{Type: t, Start: loc[0], End: loc[1]})
	next:
	}
	return out
}

// matchDictionary busca coincidencias del diccionario con frontera de palabra.
func (d *Detector) matchDictionary(text string) []Detection {
	var out []Detection
	matches := d.trie.MatchString(strings.ToLower(text))
	for _, m := range matches {
		start := int(m.Pos())
		end := start + len(m.Match())
		if !wordBoundary(text, start, end) {
			continue
		}
		t := d.dictType[m.MatchString()]
		out = append(out, Detection{Type: t, Start: start, End: end})
	}
	return out
}

// wordBoundary comprueba que la coincidencia no esté dentro de otra palabra.
func wordBoundary(text string, start, end int) bool {
	if start > 0 && isWordRune(rune(text[start-1])) {
		return false
	}
	if end < len(text) && isWordRune(rune(text[end])) {
		return false
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isDniNIE valida DNI/NIE español mediante módulo 23.
func isDniNIE(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 9 {
		return false
	}
	num := s[:8]
	switch s[0] {
	case 'X':
		num = "0" + num[1:]
	case 'Y':
		num = "1" + num[1:]
	case 'Z':
		num = "2" + num[1:]
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return false
	}
	const letters = "TRWAGMYFPDXBNJZSQVHLCKE"
	return letters[n%23] == s[8]
}

// isIBAN valida el checksum IBAN (mod 97).
func isIBAN(s string) bool {
	s = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "-", ""))
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	rearranged := s[4:] + s[:4]
	var sb strings.Builder
	for _, r := range rearranged {
		if r >= 'A' && r <= 'Z' {
			sb.WriteString(strconv.Itoa(int(r - 'A' + 10)))
		} else {
			sb.WriteRune(r)
		}
	}
	return mod97(sb.String()) == 1
}

// isCard valida una tarjeta mediante el algoritmo de Luhn.
func isCard(s string) bool {
	s = strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "-", "")
	if len(s) < 13 || len(s) > 19 {
		return false
	}
	sum, double := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		d := int(s[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func mod97(s string) int {
	rem := 0
	for _, r := range s {
		rem = (rem*10 + int(r-'0')) % 97
	}
	return rem
}

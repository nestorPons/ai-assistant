package dashboard

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var envLineRe = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=`)

// EnvFile representa un archivo .env editable conservando comentarios y el
// orden original de las claves.
type EnvFile struct {
	path  string
	lines []string
}

// LoadEnvFile lee el archivo indicado. Si no existe devuelve un EnvFile vacío.
func LoadEnvFile(path string) (*EnvFile, error) {
	f := &EnvFile{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return nil, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		f.lines = append(f.lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return f, nil
}

// Path devuelve la ruta del archivo.
func (f *EnvFile) Path() string { return f.path }

// Get devuelve el valor de una clave (descomillado) y si existe.
func (f *EnvFile) Get(key string) (string, bool) {
	for _, line := range f.lines {
		m := envLineRe.FindStringSubmatch(line)
		if m == nil || m[1] != key {
			continue
		}
		idx := strings.Index(line, "=")
		return unquote(strings.TrimSpace(line[idx+1:])), true
	}
	return "", false
}

// Keys devuelve las claves presentes en orden de aparición.
func (f *EnvFile) Keys() []string {
	var keys []string
	for _, line := range f.lines {
		if m := envLineRe.FindStringSubmatch(line); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys
}

// Set actualiza una clave existente o la añade al final.
func (f *EnvFile) Set(key, value string) {
	entry := key + "=" + quote(value)
	for i, line := range f.lines {
		m := envLineRe.FindStringSubmatch(line)
		if m != nil && m[1] == key {
			f.lines[i] = entry
			return
		}
	}
	if len(f.lines) > 0 && strings.TrimSpace(f.lines[len(f.lines)-1]) != "" {
		f.lines = append(f.lines, "")
	}
	f.lines = append(f.lines, entry)
}

// Save escribe el archivo con permisos restringidos (0600).
func (f *EnvFile) Save() error {
	content := strings.Join(f.lines, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(f.path, []byte(content), 0o600)
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func quote(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t#\"'") {
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `"`, `\"`)
		return `"` + v + `"`
	}
	return v
}

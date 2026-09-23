package detect

import "testing"

func TestIsDniNIE(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"12345678Z", true},
		{"X1234567L", true},
		{"X1234567Z", false}, // letra correcta para ese número es L
		{"1234567Z", false},  // longitud incorrecta
		{"12345678A", false},
	}
	for _, c := range cases {
		if got := isDniNIE(c.in); got != c.want {
			t.Errorf("isDniNIE(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsIBAN(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"ES9121000418450200051332", true},
		{"ES91 2100 0418 4502 0005 1332", true},
		{"ES9121000418450200051333", false},
		{"XX", false},
	}
	for _, c := range cases {
		if got := isIBAN(c.in); got != c.want {
			t.Errorf("isIBAN(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsCard(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"4111111111111111", true},
		{"4111 1111 1111 1111", true},
		{"4111111111111112", false},
		{"123", false},
	}
	for _, c := range cases {
		if got := isCard(c.in); got != c.want {
			t.Errorf("isCard(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDetectorEmailsAndPhones(t *testing.T) {
	d := New(nil, nil, nil)
	det := d.Detect("Escribe a maria@example.com o llama al 612345678")
	types := map[EntityType]bool{}
	for _, x := range det {
		types[x.Type] = true
	}
	if !types[EntityEmail] {
		t.Error("no detectó email")
	}
	if !types[EntityPhone] {
		t.Error("no detectó teléfono")
	}
}

func TestDetectorDictionary(t *testing.T) {
	d := New([]string{"María", "Juan"}, []string{"Acme"}, []string{"Madrid"})
	det := d.Detect("María trabaja en Acme en Madrid")
	got := map[EntityType]int{}
	for _, x := range det {
		got[x.Type]++
	}
	if got[EntityPerson] != 1 || got[EntityOrg] != 1 || got[EntityLoc] != 1 {
		t.Errorf("detecciones inesperadas: %v", got)
	}
}

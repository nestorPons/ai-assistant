package dashboard

import (
	"strings"

	"github.com/rivo/tview"
)

var secretHints = []string{"TOKEN", "SECRET", "PASSWORD", "API_KEY", "CLIENT_SECRET", "REFRESH_TOKEN"}

func isSecretKey(key string) bool {
	up := strings.ToUpper(key)
	for _, h := range secretHints {
		if strings.Contains(up, h) {
			return true
		}
	}
	return false
}

type configView struct {
	d         *Dashboard
	root      *tview.Flex
	list      *tview.List
	reveal    bool
	revealBtn *tview.Button
}

func newConfigView(d *Dashboard) *configView {
	v := &configView{d: d}
	v.list = tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)
	v.list.SetBorder(true).SetTitle(" Configuración (.env) ")
	v.list.SetSelectedFunc(func(_ int, _, _ string, _ rune) { v.editSelected() })

	editBtn := tview.NewButton("Editar").SetSelectedFunc(v.editSelected)
	addBtn := tview.NewButton("Añadir").SetSelectedFunc(v.addKey)
	v.revealBtn = tview.NewButton("Mostrar secretos").SetSelectedFunc(v.toggleReveal)
	reloadBtn := tview.NewButton("Recargar").SetSelectedFunc(v.reload)

	toolbar := tview.NewFlex().
		AddItem(editBtn, 10, 0, false).
		AddItem(addBtn, 10, 0, false).
		AddItem(v.revealBtn, 20, 0, false).
		AddItem(reloadBtn, 12, 0, false)

	v.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.list, 0, 1, true).
		AddItem(toolbar, 1, 0, false)
	return v
}

func (v *configView) Primitive() tview.Primitive { return v.root }

func (v *configView) Refresh() { v.reload() }

func (v *configView) reload() {
	env, err := LoadEnvFile(v.d.envPath)
	if err != nil {
		v.d.activity.Add("error leyendo .env: %v", err)
		return
	}
	v.d.env = env

	v.list.Clear()
	for _, key := range env.Keys() {
		value, _ := env.Get(key)
		display := value
		if isSecretKey(key) && !v.reveal {
			if value == "" {
				display = "(vacío)"
			} else {
				display = strings.Repeat("•", min(len(value), 12))
			}
		}
		if display == "" {
			display = "(vacío)"
		}
		key := key
		v.list.AddItem(key, "  "+truncate(display, 90), 0, nil)
	}
}

func (v *configView) selectedKey() (string, bool) {
	if v.list.GetItemCount() == 0 {
		return "", false
	}
	idx := v.list.GetCurrentItem()
	main, _ := v.list.GetItemText(idx)
	return main, main != ""
}

func (v *configView) toggleReveal() {
	v.reveal = !v.reveal
	if v.reveal {
		v.revealBtn.SetLabel("Ocultar secretos")
	} else {
		v.revealBtn.SetLabel("Mostrar secretos")
	}
	v.reload()
}

func (v *configView) editSelected() {
	key, ok := v.selectedKey()
	if !ok {
		return
	}
	value, _ := v.d.env.Get(key)
	secret := isSecretKey(key)

	form := tview.NewForm().
		AddTextView("", "Variable: "+key, 0, 1, true, false)
	if secret {
		form.AddPasswordField("Valor", value, 46, '*', nil)
	} else {
		form.AddInputField("Valor", value, 46, nil, nil)
	}
	form.AddButton("Guardar", func() {
		newValue := form.GetFormItem(1).(*tview.InputField).GetText()
		if err := v.saveKey(key, newValue); err != nil {
			return
		}
		v.d.closeModal()
		v.reload()
	})
	form.AddButton("Cancelar", v.d.closeModal)
	form.SetBorder(true).SetTitle(" Editar variable ").SetTitleAlign(tview.AlignCenter)
	v.d.openModal(form, 64, 11)
}

func (v *configView) addKey() {
	var key, value string
	form := tview.NewForm().
		AddTextView("", "", 0, 1, true, false).
		AddInputField("Clave", "", 32, nil, func(s string) { key = s }).
		AddInputField("Valor", "", 46, nil, func(s string) { value = s })
	statusText := form.GetFormItem(0).(*tview.TextView)
	form.AddButton("Guardar", func() {
		key = strings.TrimSpace(strings.ToUpper(key))
		if !validEnvKey(key) {
			statusText.SetText("[red]Clave inválida (usa A-Z, 0-9 y _)[-]")
			return
		}
		if err := v.saveKey(key, value); err != nil {
			return
		}
		v.d.closeModal()
		v.reload()
	})
	form.AddButton("Cancelar", v.d.closeModal)
	form.SetBorder(true).SetTitle(" Añadir variable ").SetTitleAlign(tview.AlignCenter)
	v.d.openModal(form, 64, 13)
}

func (v *configView) saveKey(key, value string) error {
	v.d.env.Set(key, value)
	if err := v.d.env.Save(); err != nil {
		v.d.activity.Add("error guardando .env: %v", err)
		return err
	}
	v.d.activity.Add("configuración actualizada: %s", key)
	return nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0)
		if !ok {
			return false
		}
	}
	return true
}

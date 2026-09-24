package dashboard

import (
	"context"
	"errors"

	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

type clientsView struct {
	d       *Dashboard
	root    *tview.Flex
	table   *tview.Table
	filter  *tview.Button
	items   []domain.Client
	showAll bool
}

func newClientsView(d *Dashboard) *clientsView {
	v := &clientsView{d: d}
	v.table = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	v.table.SetBorder(true).SetTitle(" Clientes autorizados ")
	v.table.SetSelectedFunc(func(_, _ int) { v.editSelected() })

	newBtn := tview.NewButton("Nuevo").SetSelectedFunc(v.newClient)
	editBtn := tview.NewButton("Editar").SetSelectedFunc(v.editSelected)
	activeBtn := tview.NewButton("Activar/Desactivar").SetSelectedFunc(v.toggleActive)
	trackBtn := tview.NewButton("Rastrear").SetSelectedFunc(v.toggleTracked)
	v.filter = tview.NewButton("Ver todos").SetSelectedFunc(v.toggleFilter)
	reloadBtn := tview.NewButton("Recargar").SetSelectedFunc(v.Refresh)

	toolbar := tview.NewFlex().
		AddItem(newBtn, 10, 0, false).
		AddItem(editBtn, 10, 0, false).
		AddItem(activeBtn, 22, 0, false).
		AddItem(trackBtn, 12, 0, false).
		AddItem(v.filter, 14, 0, false).
		AddItem(reloadBtn, 12, 0, false)

	v.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.table, 0, 1, true).
		AddItem(toolbar, 1, 0, false)
	return v
}

func (v *clientsView) Primitive() tview.Primitive { return v.root }

func (v *clientsView) Refresh() {
	clients, err := v.d.store.ListClients(context.Background(), !v.showAll)
	if err != nil {
		v.d.activity.Add("error listando clientes: %v", err)
		return
	}
	v.items = clients

	if v.showAll {
		v.filter.SetLabel("Ver rastreados")
	} else {
		v.filter.SetLabel("Ver todos")
	}

	v.table.Clear()
	for c, h := range []string{"Nombre", "Canal", "Identificador", "Rastreado", "Activo", "Creado"} {
		v.table.SetCell(0, c, headerCell(h))
	}
	for i, cl := range clients {
		row := i + 1
		v.table.SetCell(row, 0, textCell(cl.Name))
		v.table.SetCell(row, 1, textCell(string(cl.Source)))
		v.table.SetCell(row, 2, textCell(cl.Identifier))
		v.table.SetCell(row, 3, boolCell(cl.Tracked))
		v.table.SetCell(row, 4, boolCell(cl.Active))
		v.table.SetCell(row, 5, textCell(cl.CreatedAt.Format("2006-01-02 15:04")))
	}
	if len(clients) > 0 {
		v.table.Select(1, 0)
	}
}

func (v *clientsView) selected() (*domain.Client, bool) {
	row, _ := v.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(v.items) {
		return nil, false
	}
	return &v.items[idx], true
}

func (v *clientsView) toggleFilter() {
	v.showAll = !v.showAll
	v.Refresh()
}

func (v *clientsView) toggleActive() {
	c, ok := v.selected()
	if !ok {
		return
	}
	c.Active = !c.Active
	if err := v.d.store.UpdateClient(context.Background(), c); err != nil {
		v.d.activity.Add("error actualizando cliente: %v", err)
		return
	}
	v.d.activity.Add("cliente %s activo=%v", c.Identifier, c.Active)
	v.Refresh()
}

func (v *clientsView) toggleTracked() {
	c, ok := v.selected()
	if !ok {
		return
	}
	c.Tracked = !c.Tracked
	if err := v.d.store.UpdateClient(context.Background(), c); err != nil {
		v.d.activity.Add("error actualizando cliente: %v", err)
		return
	}
	v.d.activity.Add("cliente %s rastreado=%v", c.Identifier, c.Tracked)
	v.Refresh()
}

func (v *clientsView) newClient() {
	var (
		name, identifier, source string
		tracked, active          = true, true
	)
	form := tview.NewForm().
		AddTextView("", "", 0, 1, true, false).
		AddInputField("Nombre", "", 32, nil, func(t string) { name = t }).
		AddInputField("Identificador", "", 32, nil, func(t string) { identifier = t }).
		AddDropDown("Canal", []string{"gmail", "telegram", "whatsapp"}, 0, func(o string, _ int) { source = o }).
		AddCheckbox("Rastreado", true, func(c bool) { tracked = c }).
		AddCheckbox("Activo", true, func(c bool) { active = c })
	source = "gmail"
	status := form.GetFormItem(0).(*tview.TextView)

	form.AddButton("Guardar", func() {
		client := &domain.Client{
			Source:     domain.Source(source),
			Identifier: identifier,
			Name:       name,
			Tracked:    tracked,
			Active:     active,
		}
		if !client.Source.Valid() || client.Identifier == "" {
			status.SetText("[red]Canal o identificador inválidos[-]")
			return
		}
		err := v.d.store.CreateClient(context.Background(), client)
		if err != nil {
			if errors.Is(err, persistence.ErrDuplicate) {
				status.SetText("[red]Ya existe ese cliente en el canal[-]")
			} else {
				status.SetText("[red]Error: " + err.Error() + "[-]")
			}
			return
		}
		v.d.activity.Add("cliente creado: %s (%s/%s)", name, source, identifier)
		v.d.closeModal()
		v.Refresh()
	})
	form.AddButton("Cancelar", v.d.closeModal)
	form.SetBorder(true).SetTitle(" Nuevo cliente ").SetTitleAlign(tview.AlignCenter)

	v.d.openModal(form, 62, 18)
}

func (v *clientsView) editSelected() {
	c, ok := v.selected()
	if !ok {
		return
	}
	var name string
	tracked, active := c.Tracked, c.Active

	sourceField := tview.NewInputField().SetLabel("Canal").SetText(string(c.Source)).SetFieldWidth(32).SetDisabled(true)
	identifierField := tview.NewInputField().SetLabel("Identificador").SetText(c.Identifier).SetFieldWidth(32).SetDisabled(true)

	form := tview.NewForm().
		AddTextView("", "", 0, 1, true, false).
		AddFormItem(sourceField).
		AddFormItem(identifierField).
		AddInputField("Nombre", c.Name, 32, nil, func(t string) { name = t }).
		AddCheckbox("Rastreado", c.Tracked, func(b bool) { tracked = b }).
		AddCheckbox("Activo", c.Active, func(b bool) { active = b })
	name = c.Name
	status := form.GetFormItem(0).(*tview.TextView)

	form.AddButton("Guardar", func() {
		c.Name, c.Tracked, c.Active = name, tracked, active
		if err := v.d.store.UpdateClient(context.Background(), c); err != nil {
			status.SetText("[red]Error: " + err.Error() + "[-]")
			return
		}
		v.d.activity.Add("cliente actualizado: %s", c.Identifier)
		v.d.closeModal()
		v.Refresh()
	})
	form.AddButton("Cancelar", v.d.closeModal)
	form.SetBorder(true).SetTitle(" Editar cliente ").SetTitleAlign(tview.AlignCenter)

	v.d.openModal(form, 62, 18)
}

package dashboard

import (
	"context"
	"fmt"

	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

type messagesView struct {
	d     *Dashboard
	root  *tview.Flex
	table *tview.Table
	items []domain.RawMessage
}

func newMessagesView(d *Dashboard) *messagesView {
	v := &messagesView{d: d}
	v.table = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	v.table.SetBorder(true).SetTitle(" Mensajes ingeridos ")
	v.table.SetSelectedFunc(func(_, _ int) { v.detailSelected() })

	detailBtn := tview.NewButton("Ver detalle").SetSelectedFunc(v.detailSelected)
	reloadBtn := tview.NewButton("Recargar").SetSelectedFunc(v.Refresh)

	toolbar := tview.NewFlex().
		AddItem(detailBtn, 14, 0, false).
		AddItem(reloadBtn, 12, 0, false)

	v.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.table, 0, 1, true).
		AddItem(toolbar, 1, 0, false)
	return v
}

func (v *messagesView) Primitive() tview.Primitive { return v.root }

func (v *messagesView) Refresh() {
	ctx := context.Background()
	msgs, err := v.d.store.ListRawMessages(ctx, "")
	if err != nil {
		v.d.activity.Add("error listando mensajes: %v", err)
		return
	}
	v.items = msgs

	names := map[string]string{}
	if clients, err := v.d.store.ListClients(ctx, false); err == nil {
		for _, c := range clients {
			names[c.ID] = c.Name
			if names[c.ID] == "" {
				names[c.ID] = c.Identifier
			}
		}
	}

	v.table.Clear()
	for c, h := range []string{"Fecha", "Canal", "Cliente", "Procesado", "Contenido"} {
		v.table.SetCell(0, c, headerCell(h))
	}
	for i, m := range msgs {
		row := i + 1
		who := names[m.ClientID]
		if who == "" {
			who = m.ClientID
		}
		v.table.SetCell(row, 0, textCell(m.CreatedAt.Format("2006-01-02 15:04")))
		v.table.SetCell(row, 1, textCell(string(m.Source)))
		v.table.SetCell(row, 2, textCell(who))
		v.table.SetCell(row, 3, boolCell(m.Processed))
		v.table.SetCell(row, 4, textCell(truncate(m.Content, 80)))
	}
	if len(msgs) > 0 {
		v.table.Select(1, 0)
	}
}

func (v *messagesView) selected() (*domain.RawMessage, bool) {
	row, _ := v.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(v.items) {
		return nil, false
	}
	return &v.items[idx], true
}

func (v *messagesView) detailSelected() {
	m, ok := v.selected()
	if !ok {
		return
	}
	header := fmt.Sprintf("[yellow]ID:[-] %s   [yellow]Canal:[-] %s   [yellow]Procesado:[-] %v\n[yellow]Cliente:[-] %s   [yellow]Fecha:[-] %s\n\n",
		m.ID, m.Source, m.Processed, m.ClientID, m.CreatedAt.Format("2006-01-02 15:04:05"))
	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	tv.SetText(header + m.Content)
	tv.SetBorder(true).SetTitle(" Detalle del mensaje ").SetTitleAlign(tview.AlignCenter)
	v.d.openModal(tv, 84, 22)
}

package dashboard

import (
	"context"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

type homeView struct {
	d    *Dashboard
	root *tview.TextView
}

func newHomeView(d *Dashboard) *homeView {
	v := &homeView{d: d}
	v.root = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	v.root.SetBorder(true).SetTitle(" Resumen ")
	return v
}

func (v *homeView) Primitive() tview.Primitive { return v.root }

func (v *homeView) Refresh() {
	ctx := context.Background()
	var b strings.Builder

	clients, _ := v.d.store.ListClients(ctx, false)
	tracked := 0
	for _, c := range clients {
		if c.Tracked && c.Active {
			tracked++
		}
	}

	tasks, _ := v.d.store.ListTasks(ctx, nil)
	byStatus := map[domain.TaskStatus]int{}
	byPriority := map[domain.Priority]int{}
	for _, t := range tasks {
		byStatus[t.Status]++
		byPriority[t.Priority]++
	}

	messages, _ := v.d.store.ListRawMessages(ctx, "")
	pending := 0
	for _, m := range messages {
		if !m.Processed {
			pending++
		}
	}

	fmt.Fprintf(&b, " [yellow::b]Clientes[-:-:-]\n")
	fmt.Fprintf(&b, "   totales: %d   autorizados (rastreado+activo): [green]%d[-]\n\n", len(clients), tracked)

	fmt.Fprintf(&b, " [yellow::b]Tareas[-:-:-]\n")
	fmt.Fprintf(&b, "   totales: %d\n", len(tasks))
	for _, st := range []domain.TaskStatus{
		domain.TaskStatusPending, domain.TaskStatusInProgress,
		domain.TaskStatusNeedsReview, domain.TaskStatusCompleted, domain.TaskStatusDiscarded,
	} {
		fmt.Fprintf(&b, "   [%s]%s[-]: %d\n", statusTag(st), st, byStatus[st])
	}
	fmt.Fprintf(&b, "   prioridad alta: [red]%d[-]  media: %d  baja: %d\n\n",
		byPriority[domain.PriorityHigh], byPriority[domain.PriorityMedium], byPriority[domain.PriorityLow])

	fmt.Fprintf(&b, " [yellow::b]Mensajes[-:-:-]\n")
	fmt.Fprintf(&b, "   totales: %d   sin procesar: [orange]%d[-]\n\n", len(messages), pending)

	fmt.Fprintf(&b, " [gray]Backend: %s · .env: %s[-]\n", v.d.backend, v.d.envPath)

	v.root.SetText(b.String())
}

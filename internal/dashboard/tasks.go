package dashboard

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

var taskStatuses = []domain.TaskStatus{
	domain.TaskStatusPending,
	domain.TaskStatusInProgress,
	domain.TaskStatusNeedsReview,
	domain.TaskStatusCompleted,
	domain.TaskStatusDiscarded,
}

type tasksView struct {
	d      *Dashboard
	root   *tview.Flex
	table  *tview.Table
	items  []domain.Task
	filter *domain.TaskStatus
}

func newTasksView(d *Dashboard) *tasksView {
	v := &tasksView{d: d}
	v.table = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	v.table.SetBorder(true).SetTitle(" Tareas ")
	v.table.SetSelectedFunc(func(_, _ int) { v.editSelected() })

	options := []string{"todas"}
	for _, s := range taskStatuses {
		options = append(options, string(s))
	}
	dropdown := tview.NewDropDown().SetLabel("Estado: ").SetOptions(options, func(opt string, _ int) {
		if opt == "todas" {
			v.filter = nil
		} else {
			st := domain.TaskStatus(opt)
			v.filter = &st
		}
		v.Refresh()
	})

	editBtn := tview.NewButton("Editar").SetSelectedFunc(v.editSelected)
	statusBtn := tview.NewButton("Cambiar estado").SetSelectedFunc(v.changeStatus)
	reloadBtn := tview.NewButton("Recargar").SetSelectedFunc(v.Refresh)

	toolbar := tview.NewFlex().
		AddItem(dropdown, 26, 0, false).
		AddItem(editBtn, 10, 0, false).
		AddItem(statusBtn, 18, 0, false).
		AddItem(reloadBtn, 12, 0, false)

	v.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(toolbar, 1, 0, false).
		AddItem(v.table, 0, 1, true)
	return v
}

func (v *tasksView) Primitive() tview.Primitive { return v.root }

func (v *tasksView) Refresh() {
	tasks, err := v.d.store.ListTasks(context.Background(), v.filter)
	if err != nil {
		v.d.activity.Add("error listando tareas: %v", err)
		return
	}
	v.items = tasks

	v.table.Clear()
	for c, h := range []string{"Título", "Estado", "Prioridad", "Confianza", "Horas", "Creado"} {
		v.table.SetCell(0, c, headerCell(h))
	}
	for i, t := range tasks {
		row := i + 1
		v.table.SetCell(row, 0, textCell(t.Title))
		v.table.SetCell(row, 1, tview.NewTableCell(string(t.Status)).SetTextColor(statusColor(t.Status)).SetExpansion(1))
		v.table.SetCell(row, 2, tview.NewTableCell(string(t.Priority)).SetTextColor(priorityColor(t.Priority)).SetExpansion(1))
		v.table.SetCell(row, 3, textCell(fmt.Sprintf("%.0f%%", t.AIConfidence*100)))
		hours := "-"
		if t.EstimatedHours != nil {
			hours = fmt.Sprintf("%.1f", *t.EstimatedHours)
		}
		v.table.SetCell(row, 4, textCell(hours))
		v.table.SetCell(row, 5, textCell(t.CreatedAt.Format("2006-01-02 15:04")))
	}
	if len(tasks) > 0 {
		v.table.Select(1, 0)
	}
}

func (v *tasksView) selected() (*domain.Task, bool) {
	row, _ := v.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(v.items) {
		return nil, false
	}
	return &v.items[idx], true
}

func (v *tasksView) editSelected() {
	t, ok := v.selected()
	if !ok {
		return
	}
	var title, description, hoursStr string
	priority, status := string(t.Priority), string(t.Status)

	form := tview.NewForm().
		AddTextView("", "", 0, 1, true, false).
		AddInputField("Título", t.Title, 44, nil, func(s string) { title = s }).
		AddTextArea("Descripción", t.Description, 44, 5, 0, func(s string) { description = s }).
		AddDropDown("Prioridad", []string{"low", "medium", "high"}, indexOfString([]string{"low", "medium", "high"}, string(t.Priority)), func(o string, _ int) { priority = o }).
		AddDropDown("Estado", statusStrings(), indexOfString(statusStrings(), string(t.Status)), func(o string, _ int) { status = o }).
		AddInputField("Horas est.", hoursStr, 10, nil, func(s string) { hoursStr = s })
	title, description = t.Title, t.Description
	if t.EstimatedHours != nil {
		hoursStr = strconv.FormatFloat(*t.EstimatedHours, 'f', -1, 64)
	}
	statusText := form.GetFormItem(0).(*tview.TextView)

	form.AddButton("Guardar", func() {
		t.Title, t.Description = title, description
		t.Priority = domain.Priority(priority)
		t.Status = domain.TaskStatus(status)
		if strings.TrimSpace(hoursStr) != "" {
			h, err := strconv.ParseFloat(strings.TrimSpace(hoursStr), 64)
			if err != nil {
				statusText.SetText("[red]Horas inválidas[-]")
				return
			}
			t.EstimatedHours = &h
		} else {
			t.EstimatedHours = nil
		}
		if !t.Priority.Valid() || !t.Status.Valid() {
			statusText.SetText("[red]Prioridad o estado inválidos[-]")
			return
		}
		if err := v.d.store.UpdateTask(context.Background(), t); err != nil {
			statusText.SetText("[red]Error: " + err.Error() + "[-]")
			return
		}
		v.d.activity.Add("tarea actualizada: %s", t.ID)
		v.d.closeModal()
		v.Refresh()
	})
	form.AddButton("Cancelar", v.d.closeModal)
	form.SetBorder(true).SetTitle(" Editar tarea ").SetTitleAlign(tview.AlignCenter)

	v.d.openModal(form, 66, 22)
}

func (v *tasksView) changeStatus() {
	t, ok := v.selected()
	if !ok {
		return
	}
	status := string(t.Status)
	form := tview.NewForm().
		AddTextView("", "Cambiar estado de:\n"+t.Title, 0, 2, true, false).
		AddDropDown("Estado", statusStrings(), indexOfString(statusStrings(), status), func(o string, _ int) { status = o })
	form.AddButton("Guardar", func() {
		if err := v.d.store.UpdateTaskStatus(context.Background(), t.ID, domain.TaskStatus(status)); err != nil {
			v.d.activity.Add("error cambiando estado: %v", err)
			return
		}
		v.d.activity.Add("tarea %s -> %s", t.ID, status)
		v.d.closeModal()
		v.Refresh()
	})
	form.AddButton("Cancelar", v.d.closeModal)
	form.SetBorder(true).SetTitle(" Cambiar estado ").SetTitleAlign(tview.AlignCenter)
	v.d.openModal(form, 60, 12)
}

func statusStrings() []string {
	out := make([]string, 0, len(taskStatuses))
	for _, s := range taskStatuses {
		out = append(out, string(s))
	}
	return out
}

func indexOfString(list []string, value string) int {
	for i, s := range list {
		if s == value {
			return i
		}
	}
	return 0
}

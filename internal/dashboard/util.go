package dashboard

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

func headerCell(s string) *tview.TableCell {
	return tview.NewTableCell(s).SetTextColor(tcell.ColorYellow).SetSelectable(false).SetExpansion(1)
}

func textCell(s string) *tview.TableCell {
	return tview.NewTableCell(s).SetExpansion(1)
}

func boolCell(b bool) *tview.TableCell {
	if b {
		return tview.NewTableCell("sí").SetTextColor(tcell.ColorGreen)
	}
	return tview.NewTableCell("no").SetTextColor(tcell.ColorGray)
}

func statusColor(s domain.TaskStatus) tcell.Color {
	switch s {
	case domain.TaskStatusCompleted:
		return tcell.ColorGreen
	case domain.TaskStatusInProgress:
		return tcell.ColorAqua
	case domain.TaskStatusPending:
		return tcell.ColorYellow
	case domain.TaskStatusNeedsReview:
		return tcell.ColorOrange
	case domain.TaskStatusDiscarded:
		return tcell.ColorGray
	default:
		return tcell.ColorWhite
	}
}

func statusTag(s domain.TaskStatus) string {
	switch s {
	case domain.TaskStatusCompleted:
		return "green"
	case domain.TaskStatusInProgress:
		return "aqua"
	case domain.TaskStatusPending:
		return "yellow"
	case domain.TaskStatusNeedsReview:
		return "orange"
	case domain.TaskStatusDiscarded:
		return "gray"
	default:
		return "white"
	}
}

func priorityColor(p domain.Priority) tcell.Color {
	switch p {
	case domain.PriorityHigh:
		return tcell.ColorRed
	case domain.PriorityMedium:
		return tcell.ColorYellow
	default:
		return tcell.ColorGray
	}
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func newToolbar() *tview.Flex {
	return tview.NewFlex()
}

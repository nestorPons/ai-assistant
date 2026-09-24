package dashboard

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// applyTheme centraliza los colores del TUI. El fondo de los inputs (y de
// botones/checkboxes) lo marca tview.Styles.ContrastBackgroundColor.
func applyTheme() {
	tview.Styles.ContrastBackgroundColor = tcell.NewHexColor(0x303030)
}

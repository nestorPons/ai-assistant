package dashboard

import (
	"strings"
	"time"

	"github.com/rivo/tview"
)

type logsView struct {
	d  *Dashboard
	tv *tview.TextView
}

func newLogsView(d *Dashboard) *logsView {
	v := &logsView{d: d}
	v.tv = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	v.tv.SetBorder(true).SetTitle(" Logs de actividad ")
	go v.loop()
	return v
}

func (v *logsView) Primitive() tview.Primitive { return v.tv }

func (v *logsView) Refresh() {
	v.tv.SetText(strings.Join(v.d.activity.Lines(), "\n"))
	v.tv.ScrollToEnd()
}

func (v *logsView) loop() {
	if v.d.app == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-v.d.done:
			return
		case <-ticker.C:
			lines := strings.Join(v.d.activity.Lines(), "\n")
			v.d.app.QueueUpdateDraw(func() {
				v.tv.SetText(lines)
				v.tv.ScrollToEnd()
			})
		}
	}
}

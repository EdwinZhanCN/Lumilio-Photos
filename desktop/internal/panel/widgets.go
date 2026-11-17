package panel

import (
	_ "embed"

	"desktop/internal/control/dto"

	"github.com/egoist/mygo/ui"
)

var (
	//go:embed icons/general.svg
	generalSVG []byte
	//go:embed icons/server.svg
	serverSVG []byte
	//go:embed icons/storage.svg
	storageSVG []byte
	//go:embed icons/lumen.svg
	lumenSVG []byte
	//go:embed icons/updates.svg
	updatesSVG []byte

	iconGeneral = ui.MustParseSVG(generalSVG)
	iconServer  = ui.MustParseSVG(serverSVG)
	iconStorage = ui.MustParseSVG(storageSVG)
	iconLumen   = ui.MustParseSVG(lumenSVG)
	iconUpdates = ui.MustParseSVG(updatesSVG)
)

func pageHeading(c *ui.Context, title, description string) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, title).FontSize(24).Bold()
		if description != "" {
			ui.Text(c, description).TextColor(c.Theme().TextMuted)
		}
	})
}

// section is a titled group of setting rows in one bordered card.
func section(c *ui.Context, title string, rows func()) {
	theme := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(theme.TextMuted)
		ui.Column(c).Radius(10).Border(1, theme.Border).Background(theme.Surface).
			Gap(1).Dividers(1, theme.Border).Children(rows)
	})
}

// settingRow names a setting on the left and builds its control on the right.
func settingRow(c *ui.Context, title, description string, control func()) ui.Element {
	theme := c.Theme()
	return ui.Row(c).Padding(12, 16).Gap(16).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Text(c, title)
			if description != "" {
				ui.Text(c, description).FontSize(12).TextColor(theme.TextMuted)
			}
		})
		if control != nil {
			ui.Row(c).Gap(8).Children(control)
		}
	})
}

// emptyRow says what a section would list, and how to fill it.
func emptyRow(c *ui.Context, title, hint string) {
	theme := c.Theme()
	ui.Column(c).Padding(14, 16).Gap(2).Children(func() {
		ui.Text(c, title)
		if hint != "" {
			ui.Text(c, hint).FontSize(12).TextColor(theme.TextMuted)
		}
	})
}

func loadingRow(c *ui.Context, label string) {
	ui.Row(c).Gap(10).Padding(24, 0).Children(func() {
		ui.Spinner(c).Label(label)
		ui.Text(c, label).TextColor(c.Theme().TextMuted)
	})
}

func dotColor(theme *ui.Theme, color dto.DotColor) ui.Color {
	switch color {
	case dto.DotGreen:
		return theme.Success
	case dto.DotYellow:
		return theme.Warning
	case dto.DotRed:
		return theme.Danger
	}
	return theme.TextMuted
}

// pill is a status in a rounded capsule, with the dot the tray shows.
func pill(c *ui.Context, label string, color dto.DotColor) ui.Element {
	theme := c.Theme()
	return ui.Row(c).Gap(6).Padding(3, 10).Radius(999).Border(1, theme.Border).Background(theme.Background).
		Role(ui.RoleStatus).Label(label).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(dotColor(theme, color))
		ui.Text(c, label).FontSize(12).SingleLine()
	})
}

func statusBadge(c *ui.Context, presentation dto.ProcessPresentation) ui.Element {
	return pill(c, StatusLabel(presentation), presentation.Color)
}

type noticeTone int

const (
	toneNeutral noticeTone = iota
	toneWarning
	toneDanger
)

// notice is an inline message about what needs the person's attention, with
// the one action that answers it.
func notice(c *ui.Context, tone noticeTone, title, body, actionLabel string, action func()) {
	theme := c.Theme()
	border := theme.Border
	switch tone {
	case toneWarning:
		border = theme.Warning
	case toneDanger:
		border = theme.Danger
	}
	ui.Row(c).Padding(12, 16).Gap(16).Radius(10).Border(1, border).Background(theme.Surface).
		Role(ui.RoleAlert).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Text(c, title).Bold()
			if body != "" {
				ui.Text(c, body).FontSize(12).TextColor(theme.TextMuted)
			}
		})
		if actionLabel != "" && action != nil {
			ui.Button(c, actionLabel).OnClick(action)
		}
	})
}

type option[T comparable] struct {
	value T
	label string
}

// selectOf is a drop-down whose options show labels in the app's language
// and choose a stable value. It reports whether the person chose another.
func selectOf[T comparable](c *ui.Context, key string, value *T, options []option[T], label string, disabled bool) bool {
	labels := make([]string, len(options))
	chosen := ""
	for i, o := range options {
		labels[i] = o.label
		if o.value == *value {
			chosen = o.label
		}
	}
	ui.Select(c.Key(key), &chosen, labels).Label(label).Disabled(disabled).MinWidth(180)
	for _, o := range options {
		if o.label == chosen {
			if o.value == *value {
				return false
			}
			*value = o.value
			return true
		}
	}
	return false
}

// trackedButton shows the progress of the operation it started in its label.
func trackedButton(c *ui.Context, t *tracker, label, loading, success string, primary, disabled bool) ui.Element {
	text := label
	switch t.state {
	case opLoading:
		text = loading
	case opSuccess:
		text = success
	}
	keyed := c.Key("tracked:" + label)
	var button ui.Element
	if primary {
		button = ui.PrimaryButton(keyed, text)
	} else {
		button = ui.Button(keyed, text)
	}
	return button.Disabled(disabled || t.loading())
}

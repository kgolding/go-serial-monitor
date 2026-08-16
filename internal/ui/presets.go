package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/kgolding/go-serial-monitor/internal/store"
)

// presetsView shows the stored ASCII/hex snippets with quick send/edit/delete
// actions, and lets the user add new ones.
type presetsView struct {
	app     *App
	presets []store.Preset
	enabled bool

	list *widget.List
}

func newPresetsView(app *App) *presetsView {
	p := &presetsView{app: app}
	p.presets = app.store.All()
	return p
}

func (p *presetsView) CanvasObject() fyne.CanvasObject {
	p.list = widget.NewList(
		func() int { return len(p.presets) },
		func() fyne.CanvasObject {
			nameLabel := newTappableLabel(nil)
			editBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil)
			return container.NewBorder(nil, nil, nil, editBtn, nameLabel)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			preset := p.presets[id]
			row := obj.(*fyne.Container)
			nameLabel := row.Objects[0].(*tappableLabel)
			editBtn := row.Objects[1].(*widget.Button)

			col := theme.Color(theme.ColorNameForeground)
			if !p.enabled {
				col = theme.Color(theme.ColorNamePlaceHolder)
			}
			nameLabel.SetText(fmt.Sprintf("%s [%s]", preset.Name, formatLabel(preset.Format)), col, func() {
				if !p.enabled {
					return
				}
				p.send(preset)
			})
			editBtn.OnTapped = func() { p.showEditor(&preset) }
		},
	)

	addBtn := widget.NewButtonWithIcon("Add preset", theme.ContentAddIcon(), func() {
		p.showEditor(nil)
	})

	return container.NewBorder(
		nil, addBtn, nil, nil,
		p.list,
	)
}

// setEnabled toggles whether clicking a preset's name sends it (i.e.
// whether the port is currently connected).
func (p *presetsView) setEnabled(enabled bool) {
	p.enabled = enabled
	if p.list != nil {
		p.list.Refresh()
	}
}

func (p *presetsView) refresh() {
	p.presets = p.app.store.All()
	if p.list != nil {
		p.list.Refresh()
	}
}

func (p *presetsView) send(preset store.Preset) {
	data, err := buildSendBytes(preset.Format, preset.Data, preset.LineEnding)
	if err != nil {
		dialog.ShowError(err, p.app.win)
		return
	}
	if err := p.app.SendBytes(data); err != nil {
		dialog.ShowError(err, p.app.win)
	}
}

func (p *presetsView) confirmDelete(preset store.Preset) {
	dialog.ShowConfirm("Delete preset", fmt.Sprintf("Delete %q?", preset.Name), func(ok bool) {
		if !ok {
			return
		}
		if err := p.app.store.Delete(preset.ID); err != nil {
			dialog.ShowError(err, p.app.win)
			return
		}
		p.refresh()
	}, p.app.win)
}

// showEditor opens the add/edit dialog. Pass nil to create a new preset.
func (p *presetsView) showEditor(existing *store.Preset) {
	title := "Add preset"
	if existing != nil {
		title = "Edit preset"
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Name")

	dataEntry := widget.NewMultiLineEntry()
	dataEntry.Wrapping = fyne.TextWrapBreak

	var formatRadio *widget.RadioGroup
	formatRadio = widget.NewRadioGroup([]string{"ASCII", "Hex"}, func(string) {
		setDataPlaceholder(dataEntry, formatRadio)
	})
	formatRadio.Horizontal = true
	formatRadio.SetSelected("ASCII")
	setDataPlaceholder(dataEntry, formatRadio)

	leSelect := widget.NewSelect(lineEndingLabels(), nil)
	leSelect.SetSelected(lineEndingLabel(store.LineEndingNone))

	if existing != nil {
		nameEntry.SetText(existing.Name)
		formatRadio.SetSelected(formatLabel(existing.Format))
		dataEntry.SetText(existing.Data)
		leSelect.SetSelected(lineEndingLabel(existing.LineEnding))
	}

	form := widget.NewForm(
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Format", formatRadio),
		widget.NewFormItem("Data", dataEntry),
		widget.NewFormItem("Line ending", leSelect),
	)

	d := dialog.NewCustomWithoutButtons(title, form, p.app.win)
	d.Resize(fyne.NewSize(460, 400))

	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		if nameEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("name is required"), p.app.win)
			return
		}
		format := formatFromLabel(formatRadio.Selected)
		if _, err := buildSendBytes(format, dataEntry.Text, store.LineEndingNone); err != nil {
			dialog.ShowError(err, p.app.win)
			return
		}

		preset := store.Preset{
			Name:       nameEntry.Text,
			Format:     format,
			Data:       dataEntry.Text,
			LineEnding: lineEndingFromLabel(leSelect.Selected),
		}
		var err error
		if existing != nil {
			preset.ID = existing.ID
			err = p.app.store.Update(preset)
		} else {
			_, err = p.app.store.Add(preset)
		}
		if err != nil {
			dialog.ShowError(err, p.app.win)
			return
		}
		d.Hide()
		p.refresh()
	})
	saveBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", d.Hide)

	buttons := []fyne.CanvasObject{cancelBtn, saveBtn}
	if existing != nil {
		toDelete := *existing
		deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
			d.Hide()
			p.confirmDelete(toDelete)
		})
		deleteBtn.Importance = widget.DangerImportance
		buttons = append([]fyne.CanvasObject{deleteBtn}, buttons...)
	}
	d.SetButtons(buttons)

	d.Show()
}

package raygui

import (
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

/*
MODIFIED: Everything in this file is new functionality that is not present in
base raygui.
*/

var cam *rl.Camera2D

func Set2DCamera(camera *rl.Camera2D) {
	cam = camera
}

// Gets mouse position, respecting the 2D camera's transform.
func GetMousePositionWorld() rl.Vector2 {
	if cam == nil {
		return rl.GetMousePosition()
	}
	return rl.GetScreenToWorld2D(rl.GetMousePosition(), *cam)
}

func WorldRectToScreen(rec rl.Rectangle) rl.Rectangle {
	if cam == nil {
		return rec
	}
	tlScreen := rl.GetWorldToScreen2D(rl.Vector2{rec.X, rec.Y}, *cam)
	brScreen := rl.GetWorldToScreen2D(rl.Vector2{rec.X + rec.Width, rec.Y + rec.Height}, *cam)
	return rl.Rectangle{
		X:      tlScreen.X,
		Y:      tlScreen.Y,
		Width:  brScreen.X - tlScreen.X,
		Height: brScreen.Y - tlScreen.Y,
	}
}

type DropdownEx struct {
	Open bool

	options []DropdownExOption
	active  int
	str     string
}

type DropdownExOption struct {
	Name  string
	Value interface{}
}

func NewDropdownEx(opts ...DropdownExOption) DropdownEx {
	d := DropdownEx{}
	d.SetOptions(opts...)

	return d
}

func MakeDropdownExList(n int, opts ...DropdownExOption) []*DropdownEx {
	list := make([]*DropdownEx, n)
	for i := range list {
		d := NewDropdownEx(opts...)
		list[i] = &d
	}
	return list
}

func (d *DropdownEx) Do(bounds rl.Rectangle) interface{} {
	d.fixupActive()

	toggle := DropdownBox(bounds, d.str, &d.active, d.Open)
	if toggle {
		d.Open = !d.Open
	}

	if len(d.options) == 0 {
		return nil
	} else {
		return d.options[d.active].Value
	}
}

func (d *DropdownEx) GetOptions() []DropdownExOption {
	return d.options
}

func (d *DropdownEx) SetOptions(opts ...DropdownExOption) {
	var names []string
	for _, opt := range opts {
		names = append(names, opt.Name)
	}

	d.options = opts
	d.str = strings.Join(names, ";")
}

func (d *DropdownEx) fixupActive() {
	if d.active >= len(d.options) {
		d.active = len(d.options) - 1
	}
	if d.active < 0 {
		d.active = 0
	}
}

func GetOpenDropdown(dropdowns []*DropdownEx) (*DropdownEx, bool) {
	for _, other := range dropdowns {
		if other.Open {
			return other, true
		}
	}

	return nil, false
}

type TextBoxEx struct {
	Active bool
}

func NewTextBoxEx() TextBoxEx {
	return TextBoxEx{}
}

func MakeTextBoxExList(n int) []*TextBoxEx {
	list := make([]*TextBoxEx, n)
	for i := range list {
		t := NewTextBoxEx()
		list[i] = &t
	}
	return list
}

func (t *TextBoxEx) Do(bounds rl.Rectangle, text string, textSize int) (string, bool) {
	newText, toggle := TextBox(bounds, text, textSize, t.Active)
	if toggle {
		t.Active = !t.Active
	}
	return newText, toggle && !t.Active // returns true if unfocusing
}

type ScrollPanelEx struct {
	Scroll rl.Vector2
}

func (s *ScrollPanelEx) Do(
	bounds rl.Rectangle,
	content rl.Rectangle,
	draw func(scroll ScrollContext),
) {
	view := ScrollPanel(bounds, content, &s.Scroll)
	viewScreen := WorldRectToScreen(view)
	rl.BeginScissorMode(int32(viewScreen.X), int32(viewScreen.Y), int32(viewScreen.Width), int32(viewScreen.Height))
	draw(ScrollContext{
		View:   view,
		Scroll: s.Scroll,
		Start:  rl.Vector2{view.X + s.Scroll.X, view.Y + s.Scroll.Y},
	})
	rl.EndScissorMode()
}

type ScrollContext struct {
	View   rl.Rectangle
	Scroll rl.Vector2
	Start  rl.Vector2 // view X/Y + scroll X/Y
}

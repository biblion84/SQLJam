package app

import (
	"github.com/bvisness/SQLJam/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

var FilterColor = rl.NewColor(40, 204, 223, 255)

type Filter struct {
	Conditions string // TODO: whatever data we actually need for our filter UI

	// UI data
	TextBox raygui.TextBoxEx
}

func NewFilter() *Node {
	return &Node{
		Title:   "Filter",
		CanSnap: true,
		Color:   FilterColor,
		Inputs:  make([]*Node, 1),
		Data:    &Filter{},
	}
}

func (d *Filter) Update(n *Node) {
	n.UISize = rl.Vector2{360, UIFieldHeight}
}

func (d *Filter) DoUI(n *Node) {
	d.Conditions, _ = d.TextBox.Do(n.UIRect, d.Conditions, 100*zoomLevel)
}

func (d *Filter) Serialize() (res string, active bool) {
	return d.Conditions, d.TextBox.Active
}

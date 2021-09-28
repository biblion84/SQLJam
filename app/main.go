package app

import (
	"fmt"
	"github.com/bvisness/SQLJam/raygui"
	"strings"

	"github.com/bvisness/SQLJam/node"
	rl "github.com/gen2brain/raylib-go/raylib"
	_ "github.com/mattn/go-sqlite3"
)

const screenWidth = 1920
const screenHeight = 1080

var nodes []*node.Node

func Main() {
	rl.InitWindow(screenWidth, screenHeight, "SQL Jam")
	defer rl.CloseWindow()

	// much fps or not you decide
	rl.SetTargetFPS(int32(rl.GetMonitorRefreshRate(rl.GetCurrentMonitor())))

	close := openDB()
	defer close()

	// init nodes
	filmTable := node.NewTable("film", "cool_films")
	filmTable.Pos = rl.Vector2{60, 100}
	nodes = append(nodes, filmTable)

	filter := node.NewFilter([]string{"rating = 'PG'", "rental_rate < 3"})
	filter.Pos = rl.Vector2{360, 100}
	filter.Inputs[0] = filmTable
	nodes = append(nodes, filter)

	pick := node.NewPickColumns("test_alias")
	pick.Pos = rl.Vector2{660, 100}
	pick.Data.(*node.PickColumns).Cols = append(pick.Data.(*node.PickColumns).Cols, "title")
	pick.Inputs[0] = filter
	nodes = append(nodes, pick)

	// main frame loop
	rl.SetExitKey(0)
	for !rl.WindowShouldClose() {
		doFrame()
	}

	ctxTree := node.NewRecursiveGenerated(pick) // try recursive gen
	fmt.Println(ctxTree.SourceToSql())
}

var latestResult *queryResult

var cam = rl.Camera2D{
	Offset: rl.Vector2{screenWidth / 2, screenHeight / 2},
	Target: rl.Vector2{screenWidth / 2, screenHeight / 2},
	Zoom:   1,
}
var panMouseStart rl.Vector2
var panCamStart rl.Vector2

func doFrame() {
	rl.BeginDrawing()
	defer rl.EndDrawing()

	rl.ClearBackground(rl.RayWhite)

	updateDrag()

	// Move camera
	if rl.IsMouseButtonDown(rl.MouseMiddleButton) {
		mousePos := rl.GetMousePosition()
		if rl.IsMouseButtonPressed(rl.MouseMiddleButton) {
			panMouseStart = mousePos
			panCamStart = cam.Target
		}

		mouseDelta := rl.Vector2Subtract(mousePos, panMouseStart)
		cam.Target = rl.Vector2Subtract(panCamStart, mouseDelta) // camera moves opposite of mouse
	}

	rl.BeginMode2D(cam)
	defer rl.EndMode2D()

	doLayout()

	// draw lines
	for _, n := range nodes {
		if n.Snapped {
			continue
		}
		for i, input := range n.Inputs {
			rl.DrawLineBezier(input.OutputPinPos, n.InputPinPos[i], 2, rl.Black)
		}
	}

	// draw nodes
	for _, n := range nodes {
		nodeRect := n.Rect()
		rl.DrawRectangleRounded(nodeRect, 0.16, 6, n.Color)
		//rl.DrawRectangleRoundedLines(nodeRect, 0.16, 6, 2, rl.Black)

		titleBarRect := rl.Rectangle{nodeRect.X, nodeRect.Y, nodeRect.Width - 24, 24}
		previewRect := rl.Rectangle{nodeRect.X + nodeRect.Width - 24, nodeRect.Y, 24, 24}

		rl.DrawText(n.Title, int32(nodeRect.X)+6, int32(nodeRect.Y)+4, 20, rl.Black) // title bar
		rl.DrawText("P", int32(previewRect.X)+4, int32(previewRect.Y)+10, 10, rl.Black)

		for i, pinPos := range n.InputPinPos {
			if n.Snapped && i == 0 {
				continue
			}
			rl.DrawCircle(int32(pinPos.X), int32(pinPos.Y), 6, rl.Black)
		}
		if !n.Snapped {
			rl.DrawCircle(int32(n.OutputPinPos.X), int32(n.OutputPinPos.Y), 6, rl.Black)
		}

		titleHover := CheckCollisionPointRec2D(rl.GetMousePosition(), titleBarRect)
		if titleHover {
			rl.DrawText(n.SQL(false), int32(titleBarRect.X), int32(titleBarRect.Y)-22, 20, rl.Black)
		}
		if titleHover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			tryStartDrag(n, n.Pos)
		}

		if draggingThis, done, canceled := dragState(n); draggingThis {
			n.Snapped = false
			n.Pos = dragNewPosition()
			if done {
				if canceled {
					n.Pos = dragObjStart
				} else {
					trySnapNode(n)
				}
			}
		}

		previewHover := CheckCollisionPointRec2D(rl.GetMousePosition(), previewRect)
		if previewHover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			latestResult = doQuery(n.SQL(false))
		}
	}

	if latestResult != nil {
		rowPos := rl.Vector2{60, 400}
		for i := -1; i < len(latestResult.Rows); i++ {
			if i < 0 {
				// print headers
				rl.DrawText(strings.Join(latestResult.Columns, "    "), int32(rowPos.X), int32(rowPos.Y), 20, rl.Black)
			} else {
				row := latestResult.Rows[i]
				valStrings := make([]string, len(row))
				for i, v := range row {
					valStrings[i] = fmt.Sprintf("%v", v)
				}
				rl.DrawText(strings.Join(valStrings, "    "), int32(rowPos.X), int32(rowPos.Y), 20, rl.Black)
			}

			rowPos.Y += 24
		}
	}

	drawToolbar()

}

func drawToolbar() {
	toolbarWidth := int32(rl.GetScreenWidth())
	toolbarHeight := int32(64)
	rl.DrawRectangle(0, 0, toolbarWidth, toolbarHeight, rl.ColorAlpha(rl.Black, 0.5))
	rl.DrawLine(0, toolbarHeight, toolbarWidth, toolbarHeight, rl.Black)
	rl.DrawLineEx(
		rl.Vector2{Y: float32(toolbarHeight)},
		rl.Vector2{X: float32(toolbarWidth), Y: float32(toolbarHeight)},
		5,
		rl.Black,
	)

	buttHeight := 40 // thicc

	if raygui.Button(rl.Rectangle{
		X:      20,
		Y:      float32(toolbarHeight / 2) - float32(buttHeight/ 2),
		Width:  100,
		Height: float32(buttHeight),
	}, "Add Table") {
		fmt.Println("Adding a Table")
	}


}

func makeDropdownOptions(opts ...string) string {
	return strings.Join(opts, ";")
}

func CheckCollisionPointRec2D(point rl.Vector2, rec rl.Rectangle) bool {
	screenRec := rl.Rectangle{
		X:      rec.X - (cam.Target.X - cam.Offset.X),
		Y:      rec.Y - (cam.Target.Y - cam.Offset.Y),
		Width:  rec.Width,
		Height: rec.Height,
	}
	return rl.CheckCollisionPointRec(point, screenRec)
}

func doLayout() {
	/*
		Layout algo is as follows:

		- Calculate heights, widths, and input pins of all unsnapped nodes
		- Calculate heights, widths, and input pins of all snapped nodes
		- Do a pass across all nodes making them wider if necessary (yay snapping!)
		- Calculate output pins and final collisions of all nodes
	*/

	const titleBarHeight = 24
	const pinStartHeight = 36
	const pinDefaultSpacing = 36
	const snapRectHeight = 30

	basicLayout := func(n *node.Node) {
		// TODO: do different stuff for different node types

		width := 280 // TODO: Dynamic width based on specific contents
		inputHeight := titleBarHeight
		outputHeight := titleBarHeight

		// init InputPinPos if necessary
		if len(n.InputPinPos) != len(n.Inputs) {
			n.InputPinPos = make([]rl.Vector2, len(n.Inputs))
		}

		pinHeight := pinStartHeight
		for i := range n.Inputs {
			if n.Snapped && i == 0 {
				continue
			}

			n.InputPinPos[i] = rl.Vector2{n.Pos.X - 1, n.Pos.Y + float32(pinHeight)}
			pinHeight += pinDefaultSpacing
			inputHeight += pinDefaultSpacing
		}
		inputHeight += 10

		if !n.Snapped {
			outputHeight += pinDefaultSpacing
		}

		height := inputHeight
		if outputHeight > height {
			height = outputHeight
		}

		n.Size = rl.Vector2{float32(width), float32(height)}
	}

	// unsnapped
	for _, n := range nodes {
		if !n.Snapped {
			basicLayout(n)
		}
	}

	// snapped
	for _, n := range nodes {
		if n.Snapped {
			basicLayout(n)
			parent := n.Inputs[0]
			n.Pos = rl.Vector2{parent.Pos.X, parent.Pos.Y + parent.Size.Y + 20}
		}
	}

	// fix sizing
	for _, n := range nodes {
		maxWidth := n.Size.X

		current := n
		for current != nil {
			if current.Size.X > maxWidth {
				maxWidth = current.Size.X
			}
			n.Size.X = maxWidth
			current.Size.X = maxWidth

			if len(current.Inputs) > 0 {
				current = current.Inputs[0]
			} else {
				current = nil
			}
		}
	}

	// output pin positions (unsnapped)
	for _, n := range nodes {
		if !n.Snapped {
			n.OutputPinPos = rl.Vector2{n.Pos.X + n.Size.X + 1, n.Pos.Y + float32(pinStartHeight)}
		}
	}

	// final calculations
	for _, n := range nodes {
		if n.Snapped {
			current := n
			for current != nil {
				n.OutputPinPos = current.OutputPinPos
				if len(current.Inputs) > 0 {
					current = current.Inputs[0]
				} else {
					current = nil
				}
			}
		}
		n.SnapTargetRect = rl.Rectangle{n.Pos.X, n.Pos.Y + n.Size.Y - snapRectHeight, n.Size.X, snapRectHeight}
	}
}

func trySnapNode(n *node.Node) {
	if !n.CanSnap {
		return
	}

	for _, other := range nodes {
		if n == other {
			continue
		}

		if CheckCollisionPointRec2D(rl.GetMousePosition(), other.SnapTargetRect) {
			n.Snapped = true
			n.Inputs[0] = other
			break
		}
	}
}

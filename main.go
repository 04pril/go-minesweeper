package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const (
	cellSize          = 24
	outerPadding      = 12
	topPanelHeight    = 68
	touchMoveSlopPx   = 10
	touchLongPressDur = 360 * time.Millisecond
)

type gameState int

const (
	statePlaying gameState = iota
	stateWon
	stateLost
)

type difficulty struct {
	Name  string
	W, H  int
	Mines int
}

var presets = []difficulty{
	{Name: "Beginner", W: 9, H: 9, Mines: 10},
	{Name: "Intermediate", W: 16, H: 16, Mines: 40},
	{Name: "Expert", W: 30, H: 16, Mines: 99},
}

type cell struct {
	Mine      bool
	Revealed  bool
	Flagged   bool
	Question  bool
	Adjacent  int
	Exploded  bool
	WrongFlag bool
}

type board struct {
	W, H        int
	Mines       int
	cells       [][]cell
	placed      bool
	revealedCnt int
	flagsCnt    int
}

func newBoard(w, h, mines int) *board {
	b := &board{}
	b.configure(w, h, mines)
	return b
}

func (b *board) configure(w, h, mines int) {
	b.W, b.H = w, h
	maxMines := w*h - 1
	if mines < 1 {
		mines = 1
	}
	if mines > maxMines {
		mines = maxMines
	}
	b.Mines = mines
	b.reset()
}

func (b *board) reset() {
	b.cells = make([][]cell, b.H)
	for y := range b.cells {
		b.cells[y] = make([]cell, b.W)
	}
	b.placed = false
	b.revealedCnt = 0
	b.flagsCnt = 0
}

func (b *board) in(x, y int) bool {
	return x >= 0 && y >= 0 && x < b.W && y < b.H
}

func (b *board) around(x, y int, fn func(nx, ny int)) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if b.in(nx, ny) {
				fn(nx, ny)
			}
		}
	}
}

func (b *board) placeMines(sx, sy int) {
	var candidates [][2]int
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if int(math.Abs(float64(x-sx))) <= 1 && int(math.Abs(float64(y-sy))) <= 1 {
				continue
			}
			candidates = append(candidates, [2]int{x, y})
		}
	}
	if len(candidates) < b.Mines {
		// fallback: only safe start cell
		candidates = candidates[:0]
		for y := 0; y < b.H; y++ {
			for x := 0; x < b.W; x++ {
				if x == sx && y == sy {
					continue
				}
				candidates = append(candidates, [2]int{x, y})
			}
		}
	}

	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for i := 0; i < b.Mines && i < len(candidates); i++ {
		p := candidates[i]
		b.cells[p[1]][p[0]].Mine = true
	}

	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.cells[y][x].Mine {
				continue
			}
			count := 0
			b.around(x, y, func(nx, ny int) {
				if b.cells[ny][nx].Mine {
					count++
				}
			})
			b.cells[y][x].Adjacent = count
		}
	}
	b.placed = true
}

func (b *board) reveal(x, y int) (hitMine, changed bool) {
	if !b.in(x, y) {
		return false, false
	}
	c := &b.cells[y][x]
	if c.Revealed || c.Flagged {
		return false, false
	}
	if !b.placed {
		b.placeMines(x, y)
	}

	if c.Mine {
		c.Revealed = true
		c.Exploded = true
		return true, true
	}

	queue := [][2]int{{x, y}}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		cx, cy := p[0], p[1]
		cc := &b.cells[cy][cx]
		if cc.Revealed || cc.Flagged {
			continue
		}
		cc.Revealed = true
		cc.Question = false
		b.revealedCnt++
		changed = true

		if cc.Adjacent == 0 {
			b.around(cx, cy, func(nx, ny int) {
				nc := &b.cells[ny][nx]
				if !nc.Revealed && !nc.Flagged {
					queue = append(queue, [2]int{nx, ny})
				}
			})
		}
	}

	return false, changed
}

func (b *board) toggleMark(x, y int, allowQuestion bool) bool {
	if !b.in(x, y) {
		return false
	}
	c := &b.cells[y][x]
	if c.Revealed {
		return false
	}

	switch {
	case !c.Flagged && !c.Question:
		c.Flagged = true
		b.flagsCnt++
	case c.Flagged:
		c.Flagged = false
		b.flagsCnt--
		if allowQuestion {
			c.Question = true
		}
	case c.Question:
		c.Question = false
	}
	return true
}

func (b *board) countAdjacentFlags(x, y int) int {
	count := 0
	b.around(x, y, func(nx, ny int) {
		if b.cells[ny][nx].Flagged {
			count++
		}
	})
	return count
}

func (b *board) chord(x, y int) (hitMine, changed bool) {
	if !b.in(x, y) {
		return false, false
	}
	c := b.cells[y][x]
	if !c.Revealed || c.Adjacent == 0 {
		return false, false
	}
	if b.countAdjacentFlags(x, y) != c.Adjacent {
		return false, false
	}

	b.around(x, y, func(nx, ny int) {
		nc := b.cells[ny][nx]
		if nc.Revealed || nc.Flagged {
			return
		}
		hit, ch := b.reveal(nx, ny)
		if hit {
			hitMine = true
		}
		if ch {
			changed = true
		}
	})
	return
}

func (b *board) revealAllMines() {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			c := &b.cells[y][x]
			if c.Mine {
				c.Revealed = true
			}
			if c.Flagged && !c.Mine {
				c.WrongFlag = true
			}
		}
	}
}

func (b *board) autoFlagMines() {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			c := &b.cells[y][x]
			if c.Mine && !c.Flagged {
				c.Flagged = true
				b.flagsCnt++
			}
		}
	}
}

func (b *board) isWin() bool {
	return b.revealedCnt == b.W*b.H-b.Mines
}

func (b *board) remainingMines() int {
	return b.Mines - b.flagsCnt
}

func (b *board) findSafeHint() (int, int, bool) {
	if !b.placed {
		return b.W / 2, b.H / 2, true
	}
	var options [][2]int
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			c := b.cells[y][x]
			if c.Revealed || c.Flagged || c.Mine {
				continue
			}
			options = append(options, [2]int{x, y})
		}
	}
	if len(options) == 0 {
		return 0, 0, false
	}
	p := options[rand.Intn(len(options))]
	return p[0], p[1], true
}

type point struct{ X, Y int }

type touchStart struct {
	X, Y         int
	LastX, LastY int
	At           time.Time
}

type theme struct {
	Name           string
	BG             color.Color
	Panel          color.Color
	Light          color.Color
	Dark           color.Color
	CellHidden     color.Color
	CellRevealed   color.Color
	CellGrid       color.Color
	CellText       color.Color
	Mine           color.Color
	Flag           color.Color
	WrongFlag      color.Color
	Accent         color.Color
	Overlay        color.Color
	Digit          color.Color
	HeaderText     color.Color
	HeaderTextSoft color.Color
}

var themes = []theme{
	{
		Name:           "Classic",
		BG:             rgb(192, 192, 192),
		Panel:          rgb(192, 192, 192),
		Light:          rgb(255, 255, 255),
		Dark:           rgb(128, 128, 128),
		CellHidden:     rgb(192, 192, 192),
		CellRevealed:   rgb(214, 214, 214),
		CellGrid:       rgb(155, 155, 155),
		CellText:       rgb(15, 15, 15),
		Mine:           rgb(10, 10, 10),
		Flag:           rgb(210, 32, 32),
		WrongFlag:      rgb(180, 0, 0),
		Accent:         rgb(32, 128, 255),
		Overlay:        color.RGBA{0, 0, 0, 120},
		Digit:          rgb(215, 40, 40),
		HeaderText:     rgb(12, 12, 12),
		HeaderTextSoft: rgb(30, 30, 30),
	},
	{
		Name:           "Dark",
		BG:             rgb(34, 36, 42),
		Panel:          rgb(48, 51, 60),
		Light:          rgb(78, 82, 93),
		Dark:           rgb(18, 20, 26),
		CellHidden:     rgb(62, 66, 78),
		CellRevealed:   rgb(86, 90, 102),
		CellGrid:       rgb(30, 33, 41),
		CellText:       rgb(242, 242, 245),
		Mine:           rgb(245, 245, 245),
		Flag:           rgb(255, 88, 88),
		WrongFlag:      rgb(255, 25, 25),
		Accent:         rgb(107, 199, 255),
		Overlay:        color.RGBA{0, 0, 0, 140},
		Digit:          rgb(255, 98, 98),
		HeaderText:     rgb(245, 245, 245),
		HeaderTextSoft: rgb(215, 215, 225),
	},
}

var numberColors = []color.Color{
	color.RGBA{},
	rgb(25, 25, 220),
	rgb(0, 130, 0),
	rgb(210, 20, 20),
	rgb(0, 0, 135),
	rgb(130, 0, 0),
	rgb(0, 128, 128),
	rgb(0, 0, 0),
	rgb(110, 110, 110),
}

type customConfig struct {
	W, H, Mines int
	field       int
}

type game struct {
	b              *board
	state          gameState
	diff           difficulty
	themeIdx       int
	allowQuestion  bool
	showHelp       bool
	showScores     bool
	showCustom     bool
	custom         customConfig
	hint           *point
	timerStart     time.Time
	pauseStarted   time.Time
	paused         bool
	elapsedSeconds int
	bestScores     map[string]int
	faceRect       image.Rectangle
	fontMain       font.Face
	touchStarts    map[ebiten.TouchID]touchStart
}

func newGame() *game {
	g := &game{
		diff:          presets[0],
		themeIdx:      0,
		allowQuestion: true,
		fontMain:      basicfont.Face7x13,
		bestScores:    loadScores(),
		touchStarts:   map[ebiten.TouchID]touchStart{},
	}
	g.b = newBoard(g.diff.W, g.diff.H, g.diff.Mines)
	g.custom = customConfig{W: 24, H: 20, Mines: 99, field: 0}
	g.reset(false)
	g.resizeWindow()
	return g
}

func (g *game) reset(changeDiff bool) {
	if changeDiff {
		g.b.configure(g.diff.W, g.diff.H, g.diff.Mines)
		g.resizeWindow()
	} else {
		g.b.reset()
	}
	g.state = statePlaying
	g.timerStart = time.Time{}
	g.pauseStarted = time.Time{}
	g.paused = false
	g.elapsedSeconds = 0
	g.hint = nil
}

func (g *game) resizeWindow() {
	w, h := g.Layout(0, 0)
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowTitle(fmt.Sprintf("Go Minesweeper - %s", g.diff.Name))
}

func (g *game) Layout(_, _ int) (int, int) {
	return g.b.W*cellSize + outerPadding*2, topPanelHeight + g.b.H*cellSize + outerPadding*2
}

func (g *game) setDifficulty(d difficulty) {
	g.diff = d
	g.reset(true)
}

func (g *game) onGameWon() {
	g.state = stateWon
	g.b.autoFlagMines()
	if !g.timerStart.IsZero() {
		elapsed := g.elapsedSeconds
		if elapsed <= 0 {
			elapsed = 1
		}
		key := g.scoreKey()
		best, ok := g.bestScores[key]
		if !ok || best == 0 || elapsed < best {
			g.bestScores[key] = elapsed
			saveScores(g.bestScores)
		}
	}
}

func (g *game) scoreKey() string {
	return fmt.Sprintf("%s_%dx%d_%d", g.diff.Name, g.diff.W, g.diff.H, g.diff.Mines)
}

func (g *game) boardPosFromCursor(mx, my int) (int, int, bool) {
	bx0, by0 := outerPadding, topPanelHeight
	if mx < bx0 || my < by0 {
		return 0, 0, false
	}
	x := (mx - bx0) / cellSize
	y := (my - by0) / cellSize
	if !g.b.in(x, y) {
		return 0, 0, false
	}
	return x, y, true
}

func (g *game) handleRevealAt(mx, my int) bool {
	if pointInRect(mx, my, g.faceRect) {
		g.reset(false)
		return true
	}

	if g.showHelp {
		g.showHelp = false
		return true
	}
	if g.showScores {
		g.showScores = false
		return true
	}

	if g.paused || g.state != statePlaying {
		return false
	}

	x, y, ok := g.boardPosFromCursor(mx, my)
	if !ok {
		return false
	}

	var hit, changed bool
	if g.b.cells[y][x].Revealed {
		hit, changed = g.b.chord(x, y)
	} else {
		hit, changed = g.b.reveal(x, y)
	}

	if changed && g.timerStart.IsZero() && g.b.placed {
		g.timerStart = time.Now()
	}
	if changed {
		g.hint = nil
	}

	if hit {
		g.state = stateLost
		g.b.revealAllMines()
		return true
	}
	if g.b.isWin() {
		g.onGameWon()
	}
	return changed
}

func (g *game) handleMarkAt(mx, my int) bool {
	if g.paused || g.state != statePlaying || g.showHelp || g.showScores {
		return false
	}
	x, y, ok := g.boardPosFromCursor(mx, my)
	if !ok {
		return false
	}
	if g.b.toggleMark(x, y, g.allowQuestion) {
		g.hint = nil
		return true
	}
	return false
}

func (g *game) handleTouchInput() {
	for _, id := range ebiten.TouchIDs() {
		x, y := ebiten.TouchPosition(id)
		st, ok := g.touchStarts[id]
		if !ok {
			g.touchStarts[id] = touchStart{X: x, Y: y, LastX: x, LastY: y, At: time.Now()}
			continue
		}
		st.LastX, st.LastY = x, y
		g.touchStarts[id] = st
	}

	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		g.touchStarts[id] = touchStart{X: x, Y: y, LastX: x, LastY: y, At: time.Now()}
	}

	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		st, ok := g.touchStarts[id]
		if !ok {
			continue
		}
		delete(g.touchStarts, id)

		dx := absInt(st.LastX - st.X)
		dy := absInt(st.LastY - st.Y)
		if dx > touchMoveSlopPx || dy > touchMoveSlopPx {
			continue
		}

		if time.Since(st.At) >= touchLongPressDur {
			g.handleMarkAt(st.LastX, st.LastY)
			continue
		}
		g.handleRevealAt(st.LastX, st.LastY)
	}
}

func (g *game) handleGlobalKeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		g.reset(false)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key1) || inpututil.IsKeyJustPressed(ebiten.KeyB) {
		g.setDifficulty(presets[0])
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) || inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.setDifficulty(presets[1])
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) || inpututil.IsKeyJustPressed(ebiten.KeyE) {
		g.setDifficulty(presets[2])
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.themeIdx = (g.themeIdx + 1) % len(themes)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.allowQuestion = !g.allowQuestion
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.showHelp = !g.showHelp
		if g.showHelp {
			g.showScores = false
			g.showCustom = false
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.showScores = !g.showScores
		if g.showScores {
			g.showHelp = false
			g.showCustom = false
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.showCustom = !g.showCustom
		if g.showCustom {
			g.showHelp = false
			g.showScores = false
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyP) && g.state == statePlaying {
		g.paused = !g.paused
		if g.paused {
			g.pauseStarted = time.Now()
		} else if !g.pauseStarted.IsZero() && !g.timerStart.IsZero() {
			g.timerStart = g.timerStart.Add(time.Since(g.pauseStarted))
			g.pauseStarted = time.Time{}
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyH) && g.state == statePlaying && !g.paused {
		x, y, ok := g.b.findSafeHint()
		if ok {
			g.hint = &point{X: x, Y: y}
		}
	}
}

func (g *game) handleCustomDialog() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.showCustom = false
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.custom.field = (g.custom.field + 2) % 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		g.custom.field = (g.custom.field + 1) % 3
	}

	delta := 0
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		delta = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		delta = -1
	}

	if delta != 0 {
		switch g.custom.field {
		case 0:
			g.custom.W = clamp(g.custom.W+delta, 9, 60)
		case 1:
			g.custom.H = clamp(g.custom.H+delta, 9, 32)
		case 2:
			maxM := g.custom.W*g.custom.H - 1
			g.custom.Mines = clamp(g.custom.Mines+delta, 10, maxM)
		}
		maxM := g.custom.W*g.custom.H - 1
		if g.custom.Mines > maxM {
			g.custom.Mines = maxM
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.setDifficulty(difficulty{
			Name:  "Custom",
			W:     g.custom.W,
			H:     g.custom.H,
			Mines: g.custom.Mines,
		})
		g.showCustom = false
	}
}

func (g *game) Update() error {
	g.handleGlobalKeys()

	if g.showCustom {
		g.handleCustomDialog()
		return nil
	}

	if g.state == statePlaying && g.b.placed && !g.timerStart.IsZero() && !g.paused {
		g.elapsedSeconds = int(time.Since(g.timerStart).Seconds())
		if g.elapsedSeconds > 999 {
			g.elapsedSeconds = 999
		}
	}

	mx, my := ebiten.CursorPosition()

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.handleRevealAt(mx, my)
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.handleMarkAt(mx, my)
	}

	g.handleTouchInput()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	th := themes[g.themeIdx]
	screen.Fill(th.BG)

	windowW, _ := g.Layout(0, 0)

	// top panel (3D frame)
	drawRaisedRect(screen, outerPadding-2, 10, windowW-(outerPadding-2)*2, topPanelHeight-18, th)

	// inner panel
	ebitenutil.DrawRect(screen, float64(outerPadding+4), 16, float64(windowW-outerPadding*2-8), 40, th.Panel)

	mineVal := g.b.remainingMines()
	timerVal := g.elapsedSeconds
	drawDigital(screen, outerPadding+10, 20, mineVal, 3, th.Digit)
	drawDigital(screen, windowW-outerPadding-10-58, 20, timerVal, 3, th.Digit)

	// face button
	faceSize := 28
	faceX := windowW/2 - faceSize/2
	faceY := 20
	g.faceRect = image.Rect(faceX, faceY, faceX+faceSize, faceY+faceSize)
	drawRaisedRect(screen, faceX, faceY, faceSize, faceSize, th)
	face := ":)"
	switch g.state {
	case stateLost:
		face = "X("
	case stateWon:
		face = "B)"
	default:
		if g.paused {
			face = ":|"
		}
	}
	drawTextCentered(screen, face, g.fontMain, faceX, faceY+6, faceSize, th.HeaderText)

	// board frame
	boardX, boardY := outerPadding, topPanelHeight
	bw := g.b.W * cellSize
	bh := g.b.H * cellSize
	drawSunkenRect(screen, boardX-2, boardY-2, bw+4, bh+4, th)

	for y := 0; y < g.b.H; y++ {
		for x := 0; x < g.b.W; x++ {
			g.drawCell(screen, x, y, th)
		}
	}

	info := fmt.Sprintf("%s  [%dx%d/%d]  Theme:%s  QMark:%v", g.diff.Name, g.b.W, g.b.H, g.b.Mines, th.Name, g.allowQuestion)
	text.Draw(screen, info, g.fontMain, outerPadding, 10, th.HeaderTextSoft)

	if g.paused {
		drawOverlayPanel(screen, "PAUSED", []string{"Press P to resume"}, th)
	}
	if g.showHelp {
		lines := []string{
			"N: New game | 1/2/3: Beginner/Intermediate/Expert",
			"C: Custom board | Enter: Apply custom",
			"Left click: Reveal / Chord | Right click: Flag/?",
			"Touch: tap = reveal/chord | long-press = flag/?",
			"H: Hint | P: Pause | T: Theme | S: Scores | Q: Toggle ? marks",
			"F1: Toggle Help | Click smiley to restart",
		}
		drawOverlayPanel(screen, "HELP", lines, th)
	}
	if g.showScores {
		lines := g.scoreLines()
		drawOverlayPanel(screen, "BEST SCORES", lines, th)
	}
	if g.showCustom {
		g.drawCustomDialog(screen, th)
	}

	if g.state == stateWon {
		drawBanner(screen, "YOU WIN!", th)
	}
	if g.state == stateLost {
		drawBanner(screen, "BOOM!", th)
	}
}

func (g *game) scoreLines() []string {
	if len(g.bestScores) == 0 {
		return []string{"No records yet. Win a game to create one!"}
	}
	keys := make([]string, 0, len(g.bestScores))
	for k := range g.bestScores {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s : %ds", k, g.bestScores[k]))
	}
	lines = append(lines, "(Click or press S to close)")
	return lines
}

func (g *game) drawCustomDialog(screen *ebiten.Image, th theme) {
	w, h := g.Layout(0, 0)
	pw, ph := min(440, w-40), 210
	px, py := (w-pw)/2, (h-ph)/2
	ebitenutil.DrawRect(screen, 0, 0, float64(w), float64(h), th.Overlay)
	drawSunkenRect(screen, px, py, pw, ph, th)
	ebitenutil.DrawRect(screen, float64(px+6), float64(py+6), float64(pw-12), float64(ph-12), th.Panel)

	title := "CUSTOM BOARD"
	text.Draw(screen, title, g.fontMain, px+16, py+24, th.HeaderText)
	text.Draw(screen, "Left/Right: field  Up/Down: value  Enter: start  Esc: cancel", g.fontMain, px+16, py+44, th.HeaderTextSoft)

	labels := []string{"Width", "Height", "Mines"}
	values := []int{g.custom.W, g.custom.H, g.custom.Mines}
	for i := 0; i < 3; i++ {
		x := px + 24 + i*130
		y := py + 96
		label := labels[i]
		val := fmt.Sprintf("%d", values[i])
		if g.custom.field == i {
			label = "> " + label
		}
		text.Draw(screen, label, g.fontMain, x, y, th.HeaderText)
		text.Draw(screen, val, g.fontMain, x+18, y+28, th.Accent)
	}

	maxM := g.custom.W*g.custom.H - 1
	text.Draw(screen, fmt.Sprintf("Max mines: %d", maxM), g.fontMain, px+16, py+170, th.HeaderTextSoft)
}

func (g *game) drawCell(screen *ebiten.Image, x, y int, th theme) {
	c := g.b.cells[y][x]
	px := outerPadding + x*cellSize
	py := topPanelHeight + y*cellSize

	if c.Revealed {
		ebitenutil.DrawRect(screen, float64(px), float64(py), cellSize, cellSize, th.CellRevealed)
		vector.StrokeRect(screen, float32(px), float32(py), cellSize, cellSize, 1, th.CellGrid, false)

		if c.Mine {
			mineColor := th.Mine
			if c.Exploded {
				ebitenutil.DrawRect(screen, float64(px), float64(py), cellSize, cellSize, color.RGBA{210, 40, 40, 255})
				mineColor = color.RGBA{0, 0, 0, 255}
			}
			vector.DrawFilledCircle(screen, float32(px+cellSize/2), float32(py+cellSize/2), 6, mineColor, false)
			return
		}

		if c.Adjacent > 0 {
			col := numberColors[c.Adjacent]
			if g.themeIdx == 1 && c.Adjacent == 1 {
				col = rgb(120, 170, 255)
			}
			drawTextCentered(screen, fmt.Sprintf("%d", c.Adjacent), g.fontMain, px, py+5, cellSize, col)
		}
		if c.WrongFlag {
			vector.StrokeLine(screen, float32(px+4), float32(py+4), float32(px+cellSize-4), float32(py+cellSize-4), 2, th.WrongFlag, false)
			vector.StrokeLine(screen, float32(px+cellSize-4), float32(py+4), float32(px+4), float32(py+cellSize-4), 2, th.WrongFlag, false)
		}
		return
	}

	// Hidden
	drawRaisedRect(screen, px, py, cellSize, cellSize, th)

	if c.Flagged {
		vector.DrawFilledRect(screen, float32(px+11), float32(py+6), 2, 12, th.CellText, false)
		vector.StrokeLine(screen, float32(px+11), float32(py+6), float32(px+5), float32(py+10), 1.5, th.Flag, false)
		vector.StrokeLine(screen, float32(px+5), float32(py+10), float32(px+11), float32(py+14), 1.5, th.Flag, false)
		vector.StrokeLine(screen, float32(px+11), float32(py+6), float32(px+11), float32(py+14), 1.5, th.Flag, false)
		vector.DrawFilledRect(screen, float32(px+8), float32(py+8), 3, 4, th.Flag, false)
		vector.DrawFilledRect(screen, float32(px+7), float32(py+17), 9, 2, th.CellText, false)
	} else if c.Question {
		drawTextCentered(screen, "?", g.fontMain, px, py+5, cellSize, th.CellText)
	}

	if g.hint != nil && g.hint.X == x && g.hint.Y == y && g.state == statePlaying {
		vector.StrokeRect(screen, float32(px+2), float32(py+2), cellSize-4, cellSize-4, 2, th.Accent, false)
	}
}

func drawOverlayPanel(screen *ebiten.Image, title string, lines []string, th theme) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	ebitenutil.DrawRect(screen, 0, 0, float64(w), float64(h), th.Overlay)
	pw := min(560, w-36)
	ph := min(280, h-36)
	px, py := (w-pw)/2, (h-ph)/2
	drawSunkenRect(screen, px, py, pw, ph, th)
	ebitenutil.DrawRect(screen, float64(px+6), float64(py+6), float64(pw-12), float64(ph-12), th.Panel)

	ff := basicfont.Face7x13
	text.Draw(screen, title, ff, px+16, py+24, th.HeaderText)
	y := py + 50
	for _, ln := range lines {
		text.Draw(screen, ln, ff, px+16, y, th.HeaderText)
		y += 20
		if y > py+ph-18 {
			break
		}
	}
}

func drawBanner(screen *ebiten.Image, label string, th theme) {
	w := screen.Bounds().Dx()
	bh := 30
	ebitenutil.DrawRect(screen, float64((w-220)/2), 14, 220, float64(bh), th.Overlay)
	drawTextCentered(screen, label, basicfont.Face7x13, (w-220)/2, 22, 220, th.Accent)
}

func drawRaisedRect(screen *ebiten.Image, x, y, w, h int, th theme) {
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), th.CellHidden)
	vector.StrokeLine(screen, float32(x), float32(y), float32(x+w), float32(y), 2, th.Light, false)
	vector.StrokeLine(screen, float32(x), float32(y), float32(x), float32(y+h), 2, th.Light, false)
	vector.StrokeLine(screen, float32(x+w), float32(y), float32(x+w), float32(y+h), 2, th.Dark, false)
	vector.StrokeLine(screen, float32(x), float32(y+h), float32(x+w), float32(y+h), 2, th.Dark, false)
}

func drawSunkenRect(screen *ebiten.Image, x, y, w, h int, th theme) {
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), th.Panel)
	vector.StrokeLine(screen, float32(x), float32(y), float32(x+w), float32(y), 2, th.Dark, false)
	vector.StrokeLine(screen, float32(x), float32(y), float32(x), float32(y+h), 2, th.Dark, false)
	vector.StrokeLine(screen, float32(x+w), float32(y), float32(x+w), float32(y+h), 2, th.Light, false)
	vector.StrokeLine(screen, float32(x), float32(y+h), float32(x+w), float32(y+h), 2, th.Light, false)
}

func drawTextCentered(screen *ebiten.Image, s string, f font.Face, x, y, w int, clr color.Color) {
	b := text.BoundString(f, s)
	tw := b.Dx()
	text.Draw(screen, s, f, x+(w-tw)/2, y+13, clr)
}

func drawDigital(screen *ebiten.Image, x, y, value, digits int, clr color.Color) {
	// Box
	ebitenutil.DrawRect(screen, float64(x-3), float64(y-3), float64(digits*18+6), 28, color.RGBA{20, 20, 20, 255})

	n := value
	neg := n < 0
	if neg {
		n = -n
	}
	if n > int(math.Pow10(digits))-1 {
		n = int(math.Pow10(digits)) - 1
	}

	chars := make([]int, digits)
	for i := digits - 1; i >= 0; i-- {
		chars[i] = n % 10
		n /= 10
	}
	if neg {
		chars[0] = -1 // minus
	}
	for i := 0; i < digits; i++ {
		drawSevenSegDigit(screen, x+i*18, y, chars[i], clr)
	}
}

func drawSevenSegDigit(screen *ebiten.Image, x, y, d int, clr color.Color) {
	// Segment map: a b c d e f g (bits 0..6)
	maps := []int{
		0b1111110,
		0b0110000,
		0b1101101,
		0b1111001,
		0b0110011,
		0b1011011,
		0b1011111,
		0b1110000,
		0b1111111,
		0b1111011,
	}
	mask := 0
	if d >= 0 && d <= 9 {
		mask = maps[d]
	}
	if d == -1 {
		mask = 0b0000001 // middle only
	}

	off := color.RGBA{60, 20, 20, 255}
	seg := func(on bool, rx, ry, rw, rh float64) {
		if on {
			ebitenutil.DrawRect(screen, float64(x)+rx, float64(y)+ry, rw, rh, clr)
		} else {
			ebitenutil.DrawRect(screen, float64(x)+rx, float64(y)+ry, rw, rh, off)
		}
	}

	seg(mask&0b1000000 != 0, 3, 0, 10, 2)  // a
	seg(mask&0b0100000 != 0, 13, 2, 2, 9)  // b
	seg(mask&0b0010000 != 0, 13, 13, 2, 9) // c
	seg(mask&0b0001000 != 0, 3, 22, 10, 2) // d
	seg(mask&0b0000100 != 0, 1, 13, 2, 9)  // e
	seg(mask&0b0000010 != 0, 1, 2, 2, 9)   // f
	seg(mask&0b0000001 != 0, 3, 11, 10, 2) // g
}

func rgb(r, g, b uint8) color.Color {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func pointInRect(x, y int, r image.Rectangle) bool {
	return x >= r.Min.X && x <= r.Max.X && y >= r.Min.Y && y <= r.Max.Y
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func scoreFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "minesweeper_scores.json"
	}
	base := filepath.Join(dir, "go-minesweeper")
	_ = os.MkdirAll(base, 0o755)
	return filepath.Join(base, "scores.json")
}

func loadScores() map[string]int {
	path := scoreFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]int{}
	}
	var out map[string]int
	if err := json.Unmarshal(data, &out); err != nil || out == nil {
		return map[string]int{}
	}
	return out
}

func saveScores(scores map[string]int) {
	path := scoreFilePath()
	data, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	g := newGame()
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}

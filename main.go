package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
)

type GPUData struct {
	Name        string
	Utilization int
	Temperature float64
	Power       float64
	PowerCap    float64
	VRAMUsed    uint64
	VRAMTotal   uint64
}

// platform functions — implemented by linux.go / windows.go
type chartMode int

const (
	modeUtil chartMode = iota
	modeTemp
	modePower
	modeVRAM
)

func (m chartMode) String() string {
	switch m {
	case modeUtil:
		return " UTIL "
	case modeTemp:
		return " TEMP "
	case modePower:
		return " POWER "
	case modeVRAM:
		return " VRAM "
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// config
// ---------------------------------------------------------------------------

type Config struct {
	TitleColor   string            `json:"title_color"`
	GaugeColors  map[string]string `json:"gauges"`
	ChartColors  map[string]string `json:"charts"`
	DefaultChart string            `json:"default_chart"`
}

func defaultConfig() *Config {
	return &Config{
		TitleColor: "#e65100",
		GaugeColors: map[string]string{
			"gpu":   "default",
			"temp":  "default",
			"power": "default",
			"vram":  "default",
		},
		ChartColors: map[string]string{
			"util":  "#e65100",
			"temp":  "#e65100",
			"power": "#e65100",
			"vram":  "#e65100",
		},
		DefaultChart: "util",
	}
}

var cfg *Config

func loadConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultConfig()
	}
	path := filepath.Join(home, ".config", "amdtop", "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			def := defaultConfig()
			if j, err := json.MarshalIndent(def, "", "  "); err == nil {
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, j, 0644)
			}
			return def
		}
		return defaultConfig()
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return defaultConfig()
	}
	return &c
}

// ---------------------------------------------------------------------------
// constants
// ---------------------------------------------------------------------------

const (
	animFPS      = 60
	dataInterval = time.Second
	historyLen   = 120
)

// ---------------------------------------------------------------------------
// messages
// ---------------------------------------------------------------------------

type (
	animTick time.Time
	dataTick time.Time
)

// ---------------------------------------------------------------------------
// model
// ---------------------------------------------------------------------------

type model struct {
	spUtil, spTemp, spPower, spVRAM harmonica.Spring

	pUtil, vUtil   float64
	pTemp, vTemp   float64
	pPower, vPower float64
	pVRAM, vVRAM   float64

	tUtil, tTemp, tPower, tVRAM float64

	data      *GPUData
	err       error
	chartMode chartMode
	histUtil  []float64
	histTemp  []float64
	histPower []float64
	histVRAM  []float64
	last      time.Time
	w, h      int

	compact   bool // terminal too short for full layout
	showGauge bool // compact mode: true = gauges only, false = chart only
}

func newModel() model {
	spring := func(freq, damp float64) harmonica.Spring {
		return harmonica.NewSpring(harmonica.FPS(animFPS), freq, damp)
	}

	mode := modeUtil
	if cfg != nil {
		switch cfg.DefaultChart {
		case "temp":
			mode = modeTemp
		case "power":
			mode = modePower
		case "vram":
			mode = modeVRAM
		}
	}

	return model{
		spUtil:    spring(4.0, 0.7),
		spTemp:    spring(1.5, 1.0),
		spPower:   spring(3.0, 0.8),
		spVRAM:    spring(2.0, 0.9),
		chartMode: mode,
		last:      time.Now(),
		w:         80, h: 24,
	}
}

// ---------------------------------------------------------------------------
// bubbletea lifecycle
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(m.tickAnim(), m.tickData(), tea.EnterAltScreen)
}

func (m model) tickAnim() tea.Cmd {
	return tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg { return animTick(t) })
}

func (m model) tickData() tea.Cmd {
	return tea.Tick(dataInterval, func(t time.Time) tea.Msg { return dataTick(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.compact {
				if m.showGauge {
					m.showGauge = false
					m.chartMode = modeUtil
				} else {
					m.chartMode = (m.chartMode + 1) % 4
					if m.chartMode == modeUtil {
						m.showGauge = true
					}
				}
			} else {
				m.chartMode = (m.chartMode + 1) % 4
			}
		}

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height

		// estimate full-layout height; switch to compact if too short
		estChartH := max(min(m.w-10, 80)*3/8, 3)
		estFull := 3 + 8 + 1 + (estChartH + 2) + 2 + 2 // ≈ 18 + estChartH
		if m.h < estFull && !m.compact {
			m.compact = true
			m.showGauge = true
		} else if m.h >= estFull && m.compact {
			m.compact = false
		}

	case animTick:
		m.pUtil, m.vUtil = m.spUtil.Update(m.pUtil, m.vUtil, m.tUtil)
		m.pTemp, m.vTemp = m.spTemp.Update(m.pTemp, m.vTemp, m.tTemp)
		m.pPower, m.vPower = m.spPower.Update(m.pPower, m.vPower, m.tPower)
		m.pVRAM, m.vVRAM = m.spVRAM.Update(m.pVRAM, m.vVRAM, m.tVRAM)
		return m, m.tickAnim()

	case dataTick:
		d, err := collectGPUData()
		if err != nil {
			m.err = err
		} else {
			m.err = nil
			m.data = d
			m.tUtil = float64(d.Utilization)
			m.tTemp = d.Temperature
			m.tPower = d.Power
			if d.VRAMTotal > 0 {
				m.tVRAM = float64(d.VRAMUsed) / float64(d.VRAMTotal) * 100
			}

			m.histUtil = append(m.histUtil, float64(d.Utilization))
			m.histTemp = append(m.histTemp, d.Temperature)
			if d.PowerCap > 0 {
				m.histPower = append(m.histPower, d.Power/d.PowerCap*100)
			} else {
				m.histPower = append(m.histPower, d.Power)
			}
			if d.VRAMTotal > 0 {
				m.histVRAM = append(m.histVRAM, float64(d.VRAMUsed)/float64(d.VRAMTotal)*100)
			} else {
				m.histVRAM = append(m.histVRAM, 0)
			}

			if len(m.histUtil) > historyLen {
				m.histUtil = m.histUtil[len(m.histUtil)-historyLen:]
				m.histTemp = m.histTemp[len(m.histTemp)-historyLen:]
				m.histPower = m.histPower[len(m.histPower)-historyLen:]
				m.histVRAM = m.histVRAM[len(m.histVRAM)-historyLen:]
			}
		}
		return m, m.tickData()
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// styles (initialised in main via initStyles)
// ---------------------------------------------------------------------------

var (
	colBg      lipgloss.Color
	colSurface lipgloss.Color
	colBorder  lipgloss.Color
	colText    lipgloss.Color
	colMuted   lipgloss.Color
	colBarOff  lipgloss.Color
	colGreen   lipgloss.Color
	colYellow  lipgloss.Color
	colRed     lipgloss.Color

	sTitle lipgloss.Style
	sSub   lipgloss.Style
	sLabel lipgloss.Style
	sValue lipgloss.Style
	sHelp  lipgloss.Style
	sErr   lipgloss.Style
)

func initStyles(c *Config) {
	colBg = "#0d1117"
	colSurface = "#161b22"
	colBorder = "#30363d"
	colText = "#e6edf3"
	colMuted = "#7d8590"
	colBarOff = "#21262d"
	colGreen = "#4aa84a"
	colYellow = "#ad893d"
	colRed = "#c06868"

	titleColor := lipgloss.Color(c.TitleColor)

	sTitle = lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	sSub = lipgloss.NewStyle().Foreground(colMuted).Italic(true)
	sLabel = lipgloss.NewStyle().Foreground(colMuted).Width(7).Align(lipgloss.Right)
	sValue = lipgloss.NewStyle().Bold(true).Foreground(colText)
	sHelp = lipgloss.NewStyle().Foreground(colMuted)
	sErr = lipgloss.NewStyle().Foreground(colRed)
}

// ---------------------------------------------------------------------------
// rendering helpers
// ---------------------------------------------------------------------------

func barColor(pct float64) lipgloss.Color {
	switch {
	case pct < 50:
		return colGreen
	case pct < 80:
		return colYellow
	default:
		return colRed
	}
}

func renderBar(pct float64, w int, color lipgloss.Color) string {
	pct = math.Max(0, math.Min(100, pct))
	f := int(math.Round(pct / 100 * float64(w)))
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", f)) +
		lipgloss.NewStyle().Foreground(colBarOff).Render(strings.Repeat("░", w-f))
}

func fmtBytes(b uint64) string {
	const (
		GB = 1 << 30
		MB = 1 << 20
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.0fMB", float64(b)/float64(MB))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// miniChart renders a mini bar chart using half-block characters (▀▄█)
// for 2-pixel vertical resolution per text row. Caller should ensure
// width:height ≈ 4:3 for a proper rectangular look.
func miniChart(data []float64, width, height int, maxVal float64) string {
	if len(data) == 0 || width <= 0 || height <= 0 || maxVal <= 0 {
		return ""
	}
	n := len(data)
	offset := 0
	if n > width {
		data = data[n-width:]
		n = width
	} else {
		offset = width - n
	}

	vRes := height * 2
	pixels := make([][]bool, vRes)
	for y := range pixels {
		pixels[y] = make([]bool, width)
	}

	for x := range n {
		v := max(0, math.Min(maxVal, data[x]))
		if v <= 0 {
			continue
		}
		topY := vRes - 1 - int(v/maxVal*float64(vRes-1))
		topY = max(topY, 0)
		for dy := range vRes - topY {
			pixels[topY+dy][offset+x] = true
		}
	}

	var sb strings.Builder
	for row := range height {
		topRow := row * 2
		botRow := row*2 + 1
		for x := range width {
			top := pixels[topRow][x]
			bot := botRow < vRes && pixels[botRow][x]
			switch {
			case top && bot:
				sb.WriteRune('█')
			case top && !bot:
				sb.WriteRune('▀')
			case !top && bot:
				sb.WriteRune('▄')
			default:
				sb.WriteRune(' ')
			}
		}
		if row < height-1 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// view
// ---------------------------------------------------------------------------

func (m model) View() string {
	if m.err != nil && m.data == nil {
		return "\n  " + sErr.Render(fmt.Sprintf("Error: %v", m.err)) +
			"\n  " + sHelp.Render("Press q to quit.") + "\n"
	}

	barW := m.w - 7 - 4 - 16 // 7 label + 4 spaces + 16 value
	barW = max(10, min(60, barW))

	name := "AMD GPU"
	if m.data != nil && m.data.Name != "" {
		name = m.data.Name
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(sTitle.Render("  ⬡ AMD GPU Monitor"))
	b.WriteString("  ")
	b.WriteString(sSub.Render(name))
	b.WriteString("\n\n\n")

	valStyle := lipgloss.NewStyle().Bold(true).Foreground(colText).Width(16)

	gaugeColor := func(key string, pct float64) lipgloss.Color {
		if cfg.GaugeColors[key] == "default" {
			return barColor(pct)
		}
		return lipgloss.Color(cfg.GaugeColors[key])
	}

	renderRow := func(label string, barVal float64, display string, color lipgloss.Color) {
		b.WriteString(sLabel.Render(label))
		b.WriteString("  ")
		b.WriteString(renderBar(barVal, barW, color))
		b.WriteString("  ")
		b.WriteString(valStyle.Render(display))
		b.WriteString("\n\n")
	}

	if !m.compact || m.showGauge {
		renderRow("GPU", m.pUtil, fmt.Sprintf("%.0f%%", m.pUtil), gaugeColor("gpu", m.pUtil))
		renderRow("TEMP", m.pTemp, fmt.Sprintf("%.0f°C", m.pTemp), gaugeColor("temp", m.pTemp))

		pw := fmt.Sprintf("%.0fW", m.pPower)
		powerPct := m.pPower
		if m.data != nil && m.data.PowerCap > 0 {
			pw = fmt.Sprintf("%.0f / %.0fW", m.pPower, m.data.PowerCap)
			powerPct = m.pPower / m.data.PowerCap * 100
		}
		renderRow("POWER", powerPct, pw, gaugeColor("power", powerPct))

		vramDisplay := fmt.Sprintf("%.0f%%", m.pVRAM)
		if m.data != nil {
			vramDisplay = fmt.Sprintf("%s / %s",
				fmtBytes(m.data.VRAMUsed), fmtBytes(m.data.VRAMTotal))
		}
		renderRow("VRAM", m.pVRAM, vramDisplay, gaugeColor("vram", m.pVRAM))
	}

	if !m.compact || !m.showGauge {
	var (
		histData  []float64
		histLabel string
		histColor lipgloss.Color
	)
	switch m.chartMode {
	case modeUtil:
		histData = m.histUtil
		histLabel = " UTIL "
		histColor = lipgloss.Color(cfg.ChartColors["util"])
	case modeTemp:
		histData = m.histTemp
		histLabel = " TEMP "
		histColor = lipgloss.Color(cfg.ChartColors["temp"])
	case modePower:
		histData = m.histPower
		histLabel = " POWER "
		histColor = lipgloss.Color(cfg.ChartColors["power"])
	case modeVRAM:
		histData = m.histVRAM
		histLabel = " VRAM "
		histColor = lipgloss.Color(cfg.ChartColors["vram"])
	}

	maxW := min(m.w-10, 80)
	availH := m.h - 14
	availH = max(availH, 4)

	chartW := maxW
	chartH := chartW * 3 / 8
	chartH = max(chartH, 3)
	if chartH > availH {
		chartH = availH
		chartW = chartH * 8 / 3
	}

	chartData := histData
	if len(chartData) < 3 {
		chartData = make([]float64, chartW)
	}

	chart := miniChart(chartData, chartW, chartH, 100)
	if chart != "" {
		muted := lipgloss.NewStyle().Foreground(colMuted)
		accent := lipgloss.NewStyle().Foreground(histColor)

		chartLines := strings.Split(chart, "\n")

		var top string
		if chartW > len(histLabel) {
			totalDash := chartW - len(histLabel)
			leftDash := totalDash / 2
			rightDash := totalDash - leftDash
			top = muted.Render("┌" + strings.Repeat("─", leftDash) + histLabel + strings.Repeat("─", rightDash) + "┐")
		} else {
			top = muted.Render("┌" + strings.Repeat("─", chartW) + "┐")
		}
		bot := muted.Render("└" + strings.Repeat("─", chartW) + "┘")

		var framed strings.Builder
		framed.WriteString(top + "\n")
		for _, line := range chartLines {
			framed.WriteString(muted.Render("│"))
			framed.WriteString(accent.Render(line))
			framed.WriteString(muted.Render("│"))
			framed.WriteString("\n")
		}
		framed.WriteString(bot)

		b.WriteString("\n")
		b.WriteString(framed.String())
		b.WriteString("\n")
	}
	}

	if m.err != nil {
		b.WriteString("\n  ")
		b.WriteString(sErr.Render(fmt.Sprintf("⚠ %v", m.err)))
	}

	b.WriteString("\n\n  ")
	b.WriteString(sHelp.Render("tab switch · q quit"))
	b.WriteString("\n")

	content := b.String()
	contentH := strings.Count(content, "\n")
	padTop := max((m.h-contentH)/2, 0)
	return lipgloss.NewStyle().Width(m.w).Align(lipgloss.Center).Render(
		strings.Repeat("\n", padTop) + content)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	cfg = loadConfig()
	initStyles(cfg)

	if err := initGPU(); err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}
	defer closeGPU()

	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		os.Exit(1)
	}
}

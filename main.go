package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the application state
type Model struct {
	histogram map[string]int  // Stores log frequency data
	logLines  []string        // Stores log file lines
	viewport  viewport.Model  // For scrolling through logs
	textInput textinput.Model // For command input
	logFile   string          // Name of the log file
	width     int             // Terminal width
	height    int             // Terminal height
	minTime   time.Time       // Earliest timestamp in logs
	maxTime   time.Time       // Latest timestamp in logs
	err       error           // Stores any errors
}

func initialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter command"
	ti.Focus()

	vp := viewport.New(80, 10)
	vp.SetContent("Log output will appear here...")

	return Model{
		histogram: make(map[string]int),
		logLines:  []string{},
		viewport:  vp,
		textInput: ti,
		minTime:   time.Now(),
		maxTime:   time.Time{},
		err:       nil,
	}
}

// Parse timestamp from log line
func parseTimestamp(timestampStr string) (time.Time, error) {
	var t time.Time
	var err error
	// Try all supported formats
	for _, format := range TimestampFormats {
		t, err = time.Parse(format, timestampStr)
		if err == nil {
			return t, nil
		}
	}
	return t, fmt.Errorf("неизвестный формат таймштампа: %s", timestampStr)
}

// Load and process the log file
func loadLogFile(filename string) tea.Msg {
	file, err := os.Open(filename)
	if err != nil {
		return errorMsg{err}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	histogram := make(map[string]int)
	logLines := []string{}
	minTime := time.Now()
	maxTime := time.Time{}

	for scanner.Scan() {
		line := scanner.Text()
		logLines = append(logLines, line)

		// Parse timestamp for histogram
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		// Try different timestamp formats
		var timestamp time.Time
		var err error

		// First try just the first field
		timestampStr := fields[0]
		timestamp, err = parseTimestamp(timestampStr)

		// If that failed, try first two fields
		if err != nil && len(fields) >= 2 {
			timestampStr = fields[0] + " " + fields[1]
			timestamp, err = parseTimestamp(timestampStr)
		}

		// If that failed, try first three fields
		if err != nil && len(fields) >= 3 {
			timestampStr = fields[0] + " " + fields[1] + " " + fields[2]
			timestamp, err = parseTimestamp(timestampStr)
		}

		if err != nil {
			// Skip if we couldn't parse the timestamp
			continue
		}

		// Update min and max times
		if timestamp.Before(minTime) {
			minTime = timestamp
		}
		if timestamp.After(maxTime) {
			maxTime = timestamp
		}

		// Group by minute for the histogram
		minute := timestamp.Format("2006-01-02 15:04")
		histogram[minute]++
	}

	if err := scanner.Err(); err != nil {
		return errorMsg{err}
	}

	return logFileLoadedMsg{
		histogram: histogram,
		logLines:  logLines,
		minTime:   minTime,
		maxTime:   maxTime,
	}
}

// Custom message types
type errorMsg struct{ err error }
type logFileLoadedMsg struct {
	histogram map[string]int
	logLines  []string
	minTime   time.Time
	maxTime   time.Time
}

// Implementation of tea.Model interface - Init
func (m Model) Init() tea.Cmd {
	if len(os.Args) < 2 {
		return func() tea.Msg {
			return errorMsg{fmt.Errorf("использование: go run main.go <имя_лог_файла>")}
		}
	}

	m.logFile = os.Args[1]
	return func() tea.Msg {
		return loadLogFile(m.logFile)
	}
}

// Implementation of tea.Model interface - Update
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmd := m.textInput.Value()
			switch cmd {
			case "list":
				// Show full log file
				m.viewport.SetContent(strings.Join(m.logLines, "\n"))
			case "quit", "exit":
				return m, tea.Quit
			case "help":
				m.viewport.SetContent("Доступные команды:\n" +
					"list - Показать все записи логов\n" +
					"quit - Выйти из приложения\n" +
					"help - Показать эту справку")
			default:
				m.viewport.SetContent(fmt.Sprintf("Неизвестная команда: %s\nВведите 'help' для списка команд", cmd))
			}
			m.textInput.Reset()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Define styles temporarily to get border sizes
		// Note: It might be better to define these styles once in the model
		logOutputStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD"))

		inputStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(0, 1)

		// Adjust component sizes, accounting for border/padding widths
		histogramHeight := 10 // Height for histogram area
		inputHeight := 3      // Height for input area (including border)

		// Calculate available width inside borders
		viewportWidth := m.width - logOutputStyle.GetHorizontalFrameSize()
		inputWidth := m.width - inputStyle.GetHorizontalFrameSize()

		// Viewport gets the rest of the space vertically
		m.viewport.Height = m.height - histogramHeight - inputHeight
		m.viewport.Width = viewportWidth

		// Make sure textInput has correct width, accounting for label and input border/padding
		// Label "list " is 5 chars wide.
		m.textInput.Width = inputWidth - 5

	case logFileLoadedMsg:
		m.histogram = msg.histogram
		m.logLines = msg.logLines
		m.minTime = msg.minTime
		m.maxTime = msg.maxTime

		// Set initial content for viewport
		m.viewport.SetContent(fmt.Sprintf("Файл логов загружен: %s\n%d записей найдено.\nВведите 'list' для просмотра логов.",
			m.logFile, len(m.logLines)))

	case errorMsg:
		m.err = msg.err
	}

	// Update components
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// Implementation of tea.Model interface - View
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Ошибка: %v\n\nНажмите любую клавишу для выхода.", m.err)
	}

	// Define styles
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(0, 1)

	// Create histogram visualization
	histogramView := m.renderHistogram()
	// Считаем количество строк в гистограмме
	// histogramLines := strings.Count(histogramView, "\n") + 1
	// Делаем рамку заведомо выше диаграммы (например, +2 строки)
	histogramBox := borderStyle.
		Width(m.width - borderStyle.GetHorizontalFrameSize() + 2).
		// Height(histogramLines).
		// Height(0).
		Render(histogramView)

	// Create command input with "list" label
	inputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(0, 1)

	commandLabel := lipgloss.NewStyle().
		Bold(true).
		Render("cmd")

	// Ensure command input box takes full width
	commandInput := inputStyle.Width(m.width - inputStyle.GetHorizontalFrameSize() + 2).Render(commandLabel + " " + m.textInput.View())

	// Create log output view
	logOutputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD"))
	// Ensure log output box takes full width
	logOutput := logOutputStyle.Width(m.width - logOutputStyle.GetHorizontalFrameSize()).Render(m.viewport.View())

	// Combine all parts
	return lipgloss.JoinVertical(lipgloss.Left,
		histogramBox,
		commandInput,
		logOutput,
	)
}

// Render the histogram
func (m Model) renderHistogram() string {
	if len(m.histogram) == 0 {
		return "Загрузка гистограммы..."
	}

	// Sort times for consistent display
	var times []string
	for t := range m.histogram {
		times = append(times, t)
	}
	sort.Strings(times)

	// Find max count for scaling
	maxCount := 0
	for _, count := range m.histogram {
		if count > maxCount {
			maxCount = count
		}
	}

	// Calculate histogram dimensions
	histHeight := 5 // Fixed height for the bars

	// Create a simplified ASCII histogram
	var sb strings.Builder

	// Create bar representation for each time point
	for i := 0; i < histHeight; i++ {
		for _, t := range times {
			count := m.histogram[t]
			barHeight := (count * histHeight) / maxCount
			if count > 0 && barHeight == 0 {
				barHeight = 1 // Ensure at least a height of 1 for non-zero counts
			}

			// Render bars from bottom to top
			if histHeight-i-1 < barHeight {
				sb.WriteString("█")
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// Add time labels (start, middle, end)
	if len(times) > 0 {
		// Get start, middle, and end times for labels
		startLabel := times[0]
		endLabel := times[len(times)-1]
		middleLabel := ""

		if len(times) > 2 {
			middleIdx := len(times) / 2
			middleLabel = times[middleIdx]
		}

		// Create label row with proper spacing
		timeLabels := startLabel

		if middleLabel != "" {
			// Calculate spaces needed for middle label
			middlePos := len(times)/2 - len(startLabel)
			if middlePos < 1 {
				middlePos = 1
			}
			timeLabels += strings.Repeat(" ", middlePos) + middleLabel
		}

		// Calculate spaces needed for end label
		endPos := len(times) - len(timeLabels) - len(endLabel)
		if endPos < 1 {
			endPos = 1
		}
		timeLabels += strings.Repeat(" ", endPos) + endLabel

		sb.WriteString(timeLabels)
	}

	return sb.String()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Ошибка запуска программы: %v\n", err)
		os.Exit(1)
	}
}

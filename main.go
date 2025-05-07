package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp" // добавьте этот импорт
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

	filterMode bool   // <--- новое поле: режим фильтрации
	filterExpr string // <--- новое поле: последнее выражение фильтра
}

func initialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter command"
	ti.Focus()
	ti.Prompt = "" // <--- добавьте эту строку

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
			if m.filterMode {
				// Применяем фильтр
				re, err := regexp.Compile(m.textInput.Value())
				if err != nil {
					m.viewport.SetContent(fmt.Sprintf("Ошибка в регулярном выражении: %v", err))
				} else {
					m.filterExpr = m.textInput.Value()
					var filtered []string
					for _, line := range m.logLines {
						if re.MatchString(line) {
							filtered = append(filtered, line)
						}
					}
					m.viewport.SetContent(strings.Join(filtered, "\n"))
				}
				m.filterMode = false
				m.textInput.Placeholder = "Enter command"
				m.textInput.Reset()
				return m, nil
			}
			cmd := m.textInput.Value()
			switch cmd {
			case "list":
				// Show full log file
				m.viewport.SetContent(strings.Join(m.logLines, "\n"))
			case "filter":
				m.filterMode = true
				m.textInput.Placeholder = "Введите регулярное выражение"
				m.textInput.Reset()
				return m, nil
			case "statistic":
				m.viewport.SetContent(buildLogStatistics(m.logLines))
			case "quit", "exit":
				return m, tea.Quit
			case "help":
				m.viewport.SetContent("Доступные команды:\n" +
					"list - Показать все записи логов\n" +
					"goto - Перейти к указаному таймштампу\n" +
					"filter    - Отобразить строки, соответствующие регулярному выражению\n" +
					"statistic - Сформировать статистику по лог файлу\n" +
					"analyse   - Расширенный анализ лог файла\n" +
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
		m.viewport.SetContent(fmt.Sprintf(
			"Файл логов загружен: %s\n%d записей найдено.\n"+
				"Введите 'list' для просмотра логов.\n\n"+
				"Доступные команды:\n"+
				"list - Показать все записи логов\n"+
				"goto - Перейти к указаному таймштампу\n"+
				"filter    - Отобразить строки, соответствующие регулярному выражению\n"+
				"statistic - Сформировать статистику по лог файлу\n"+
				"analyse   - Расширенный анализ лог файла\n"+
				"quit - Выйти из приложения\n"+
				"help - Показать эту справку",
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
	histogramBox := borderStyle.Width(m.width - borderStyle.GetHorizontalFrameSize() + 2).Render(histogramView)

	// Create command input with label depending on mode
	inputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(0, 1)

	var labelText string
	if m.filterMode {
		labelText = lipgloss.NewStyle().Bold(true).Render("flt")
	} else {
		labelText = lipgloss.NewStyle().Bold(true).Render("cmd")
	}

	commandInput := inputStyle.Width(m.width - inputStyle.GetHorizontalFrameSize() + 2).Render(labelText + " > " + m.textInput.View())

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

	// Определяем ширину области для гистограммы (без рамок)
	histWidth := m.width - 4 // 2 символа на каждую сторону рамки

	// Получаем все времена из гистограммы и сортируем
	var times []string
	for t := range m.histogram {
		times = append(times, t)
	}
	sort.Strings(times)
	if len(times) == 0 || histWidth < 2 {
		return "Недостаточно данных для гистограммы"
	}

	// Преобразуем строки времени в time.Time
	var timePoints []time.Time
	for _, t := range times {
		parsed, err := time.Parse("2006-01-02 15:04", t)
		if err == nil {
			timePoints = append(timePoints, parsed)
		}
	}
	if len(timePoints) == 0 {
		return "Ошибка разбора времени"
	}

	// Находим минимальное и максимальное время
	startTime := timePoints[0]
	endTime := timePoints[len(timePoints)-1]
	totalDuration := endTime.Sub(startTime)
	if totalDuration == 0 {
		totalDuration = time.Minute // чтобы не делить на 0
	}

	// Разбиваем диапазон на histWidth интервалов
	binCounts := make([]int, histWidth)
	binStarts := make([]time.Time, histWidth)
	binDuration := totalDuration / time.Duration(histWidth)

	for i := 0; i < histWidth; i++ {
		binStarts[i] = startTime.Add(time.Duration(i) * binDuration)
	}

	// Для каждой точки времени определяем, в какой интервал она попадает
	for tStr, count := range m.histogram {
		t, err := time.Parse("2006-01-02 15:04", tStr)
		if err != nil {
			continue
		}
		binIdx := int(t.Sub(startTime) / binDuration)
		if binIdx >= histWidth {
			binIdx = histWidth - 1
		}
		binCounts[binIdx] += count
	}

	// Находим максимальное значение для масштабирования
	maxCount := 0
	for _, c := range binCounts {
		if c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}

	histHeight := 5
	var sb strings.Builder

	// Рисуем столбцы
	for i := 0; i < histHeight; i++ {
		for _, count := range binCounts {
			barHeight := (count * histHeight) / maxCount
			if count > 0 && barHeight == 0 {
				barHeight = 1
			}
			if histHeight-i-1 < barHeight {
				sb.WriteString("█")
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// Подписи: начало, середина, конец
	startLabel := startTime.Format("2006-01-02 15:04")
	midLabel := startTime.Add(totalDuration / 2).Format("2006-01-02 15:04")
	endLabel := endTime.Format("2006-01-02 15:04")

	// Размещаем подписи равномерно
	labelRow := make([]rune, histWidth)
	for i := range labelRow {
		labelRow[i] = ' '
	}
	copy(labelRow, []rune(startLabel))
	midPos := histWidth/2 - len(midLabel)/2
	if midPos+len(midLabel) < histWidth {
		copy(labelRow[midPos:], []rune(midLabel))
	}
	endPos := histWidth - len(endLabel)
	if endPos >= 0 {
		copy(labelRow[endPos:], []rune(endLabel))
	}
	sb.WriteString(string(labelRow))

	return sb.String()
}

// buildLogStatistics формирует статистику по logLines и возвращает строку для отображения
func buildLogStatistics(logLines []string) string {
	type spike struct {
		Timestamp time.Time
		Count     int
	}
	var (
		totalLines       int
		linesWithTS      int
		errWrnLines      int
		otherLines       int
		spikes           []spike
		longestLine      string
		maxLen           int
		noTimestampLines int
		firstTS, lastTS  time.Time
		firstTSset       bool
		histogram        = make(map[time.Time]int)
		reErrWrn         = regexp.MustCompile(`(?i)\b(err|wrn|error|warn)\b`)
	)

	for _, line := range logLines {
		totalLines++
		fields := strings.Fields(line)
		var ts time.Time
		var err error
		foundTS := false
		// Пробуем все варианты: 1, 2, 3 поля
		for i := 1; i <= 3 && i <= len(fields); i++ {
			ts, err = parseTimestamp(strings.Join(fields[:i], " "))
			if err == nil {
				foundTS = true
				break
			}
		}
		if foundTS {
			linesWithTS++
			if !firstTSset || ts.Before(firstTS) {
				firstTS = ts
				firstTSset = true
			}
			if ts.After(lastTS) {
				lastTS = ts
			}
			minute := ts.Truncate(time.Minute)
			histogram[minute]++
		} else {
			noTimestampLines++
		}
		if reErrWrn.MatchString(line) {
			errWrnLines++
		} else {
			otherLines++
		}
		if len(line) > maxLen {
			maxLen = len(line)
			longestLine = line
		}
	}

	for t, c := range histogram {
		spikes = append(spikes, spike{t, c})
	}
	sort.Slice(spikes, func(i, j int) bool { return spikes[i].Count > spikes[j].Count })
	topSpikes := spikes
	if len(spikes) > 3 {
		topSpikes = spikes[:3]
	}
	var avgPerMin float64
	if firstTSset && lastTS.After(firstTS) {
		durationMin := lastTS.Sub(firstTS).Minutes()
		if durationMin > 0 {
			avgPerMin = float64(linesWithTS) / durationMin
		}
	}
	var sb strings.Builder
	sb.WriteString("Статистика по лог-файлу:\n")
	if firstTSset {
		sb.WriteString(fmt.Sprintf("1. Начальный таймштамп: %s\n", firstTS.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("2. Конечный таймштамп: %s\n", lastTS.Format(time.RFC3339)))
	} else {
		sb.WriteString("1-2. Нет строк с корректным таймштампом\n")
	}
	sb.WriteString(fmt.Sprintf("3. Количество строк: %d (с таймштампом: %d, без таймштампа: %d)\n", totalLines, linesWithTS, noTimestampLines))
	sb.WriteString("4. Три всплеска:\n")
	for _, s := range topSpikes {
		sb.WriteString(fmt.Sprintf("   %s — %d строк\n", s.Timestamp.Format("2006-01-02 15:04"), s.Count))
	}
	ratio := 0.0
	if totalLines > 0 {
		ratio = float64(errWrnLines) / float64(totalLines) * 100
	}
	sb.WriteString(fmt.Sprintf("5. Соотношение err/wrn к остальным: %d / %d (%.2f%%)\n", errWrnLines, otherLines, ratio))
	sb.WriteString(fmt.Sprintf("6. Среднее количество строк в минуту: %.2f\n", avgPerMin))
	sb.WriteString(fmt.Sprintf("7. Самое длинное сообщение (%d символов):\n   %s\n", maxLen, longestLine))
	return sb.String()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Ошибка запуска программы: %v\n", err)
		os.Exit(1)
	}
}

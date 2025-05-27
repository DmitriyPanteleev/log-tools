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

	filterMode bool   // режим фильтрации
	filterExpr string // последнее выражение фильтра
	gotoMode   bool   // добавьте это поле

	mainTimestampFormat string // <--- добавьте это поле
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

// Функция для определения формата первой строки с валидным таймштампом
func detectMainTimestampFormat(logLines []string) string {
	for _, line := range logLines {
		fields := strings.Fields(line)
		for n := 1; n <= 3 && n <= len(fields); n++ {
			tsStr := strings.Join(fields[:n], " ")
			for _, format := range TimestampFormats {
				if _, err := time.Parse(format, tsStr); err == nil {
					return format
				}
			}
		}
	}
	return ""
}

// Функция для дополнения таймштампа до нужного формата
func completeTimestamp(input, format string) string {
	// Дополняем нулями, если не хватает
	// Например, если format = "2006/01/02 15:04:05.000000"
	// а input = "2025/04/22 12:00", то дополняем ":00.000000"
	layoutParts := strings.Split(format, " ")
	inputParts := strings.Split(input, " ")
	for i := range inputParts {
		if len(inputParts[i]) < len(layoutParts[i]) {
			inputParts[i] += layoutParts[i][len(inputParts[i]):]
		}
	}
	// Если пользователь ввёл только дату и время без секунд, добавим ":00" и т.д.
	result := input
	if len(input) < len(format) {
		result += format[len(input):]
	}
	return result
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

	mainFormat := detectMainTimestampFormat(logLines)

	return logFileLoadedMsg{
		histogram: histogram,
		logLines:  logLines,
		minTime:   minTime,
		maxTime:   maxTime,
		// добавьте:
		mainTimestampFormat: mainFormat,
	}
}

// Custom message types
type errorMsg struct{ err error }
type logFileLoadedMsg struct {
	histogram           map[string]int
	logLines            []string
	minTime             time.Time
	maxTime             time.Time
	mainTimestampFormat string // <--- добавьте это поле
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
			if m.gotoMode {
				inputTS := m.textInput.Value()
				var target time.Time
				var parseErr error

				// Используем основной формат
				if m.mainTimestampFormat != "" {
					completedInput := completeTimestamp(inputTS, m.mainTimestampFormat)
					target, parseErr = time.Parse(m.mainTimestampFormat, completedInput)
				} else {
					parseErr = fmt.Errorf("не удалось определить формат таймштампа")
				}

				if parseErr != nil {
					m.viewport.SetContent(fmt.Sprintf("Ошибка разбора таймштампа: %v", parseErr))
				} else {
					// Найти ближайший (меньший или больший) таймштамп
					type lineWithTS struct {
						idx int
						ts  time.Time
					}
					var lines []lineWithTS
					for i, line := range m.logLines {
						fields := strings.Fields(line)
						for n := 1; n <= 3 && n <= len(fields); n++ {
							ts, err := time.Parse(m.mainTimestampFormat, strings.Join(fields[:n], " "))
							if err == nil {
								lines = append(lines, lineWithTS{i, ts})
								break
							}
						}
					}
					// Найти ближайший индекс
					bestIdx := -1
					bestDelta := time.Duration(1<<63 - 1)
					for _, l := range lines {
						delta := l.ts.Sub(target)
						if delta < 0 {
							delta = -delta
						}
						if bestIdx == -1 || delta < bestDelta || (delta == bestDelta && l.ts.After(target)) {
							bestIdx = l.idx
							bestDelta = delta
						}
					}
					if bestIdx != -1 {
						m.viewport.SetContent(strings.Join(m.logLines[bestIdx:], "\n"))
					} else {
						m.viewport.SetContent("Не найдено строк с таким или близким таймштампом")
					}
				}
				m.gotoMode = false
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
			case "stat":
				m.viewport.SetContent(buildLogStatistics(m.logLines))
			case "goto":
				m.gotoMode = true
				m.textInput.Placeholder = "Введите таймштамп"
				m.textInput.Reset()
				return m, nil
			case "analyse":
				m.viewport.SetContent(buildLogAnalysis(m.logLines))
			case "quit", "exit":
				return m, tea.Quit
			case "help":
				m.viewport.SetContent("Доступные команды:\n" +
					"list - Показать все записи логов\n" +
					"goto - Перейти к указаному таймштампу\n" +
					"filter - Отобразить строки, соответствующие регулярному выражению\n" +
					"stat - Сформировать статистику по лог файлу\n" +
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
		m.mainTimestampFormat = msg.mainTimestampFormat

		// Set initial content for viewport
		m.viewport.SetContent(fmt.Sprintf(
			"Файл логов загружен: %s\n%d записей найдено.\n"+
				"Введите 'list' для просмотра логов.\n\n"+
				"Доступные команды:\n"+
				"list - Показать все записи логов\n"+
				"goto - Перейти к указаному таймштампу\n"+
				"filter - Отобразить строки, соответствующие регулярному выражению\n"+
				"stat - Сформировать статистику по лог файлу\n"+
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
	} else if m.gotoMode {
		labelText = lipgloss.NewStyle().Bold(true).Render("gto")
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
	return sb.String()
}

// normalizeLogLine удаляет таймштамп и заменяет числа, UUID, IP на плейсхолдеры
func normalizeLogLine(line string) string {
	fields := strings.Fields(line)
	// Удаляем таймштамп (до 3-х первых полей, если это таймштамп)
	for i := 1; i <= 3 && i < len(fields); i++ {
		if _, err := parseTimestamp(strings.Join(fields[:i], " ")); err == nil {
			line = strings.Join(fields[i:], " ")
			break
		}
	}
	// Заменяем числа, UUID, IP на плейсхолдеры
	reNum := regexp.MustCompile(`\b\d+\b`)
	reUUID := regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	reIP := regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	reHex := regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
	line = reUUID.ReplaceAllString(line, "<UUID>")
	line = reIP.ReplaceAllString(line, "<IP>")
	line = reHex.ReplaceAllString(line, "<HEX>")
	line = reNum.ReplaceAllString(line, "<NUM>")
	return line
}

// buildLogAnalysis формирует сводку по самым частым паттернам сообщений
func buildLogAnalysis(logLines []string) string {
	type patternStat struct {
		Pattern string
		Count   int
		Example string
	}
	patterns := make(map[string]*patternStat)
	for _, line := range logLines {
		norm := normalizeLogLine(line)
		if stat, ok := patterns[norm]; ok {
			stat.Count++
		} else {
			patterns[norm] = &patternStat{Pattern: norm, Count: 1, Example: line}
		}
	}
	// Собираем и сортируем по убыванию
	var stats []patternStat
	for _, v := range patterns {
		stats = append(stats, *v)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Count > stats[j].Count })

	topN := 7
	if len(stats) < topN {
		topN = len(stats)
	}
	var sb strings.Builder
	sb.WriteString("Анализ лог-файла: самые частые паттерны сообщений\n")
	for i := 0; i < topN; i++ {
		sb.WriteString(fmt.Sprintf("%d. [%d раз]\n   Паттерн: %s\n   Пример: %s\n", i+1, stats[i].Count, stats[i].Pattern, stats[i].Example))
	}

	// --- Редкие паттерны ---
	var rareStats []patternStat
	for _, s := range stats {
		if s.Count <= 2 {
			rareStats = append(rareStats, s)
		}
	}
	sort.Slice(rareStats, func(i, j int) bool { return rareStats[i].Count < rareStats[j].Count })

	rareN := 5
	if len(rareStats) < rareN {
		rareN = len(rareStats)
	}
	if rareN > 0 {
		sb.WriteString("\nРедкие (уникальные или почти уникальные) паттерны:\n")
		for i := 0; i < rareN; i++ {
			sb.WriteString(fmt.Sprintf("%d. [%d раз]\n   Паттерн: %s\n   Пример: %s\n", i+1, rareStats[i].Count, rareStats[i].Pattern, rareStats[i].Example))
		}
	} else {
		sb.WriteString("\nНет уникальных или редких паттернов.\n")
	}

	// --- Новый блок: длинные сообщения ---
	type longLine struct {
		Len  int
		Line string
	}
	var longLines []longLine
	for _, line := range logLines {
		longLines = append(longLines, longLine{Len: len(line), Line: line})
	}
	sort.Slice(longLines, func(i, j int) bool { return longLines[i].Len > longLines[j].Len })

	longN := 5
	if len(longLines) < longN {
		longN = len(longLines)
	}
	if longN > 0 {
		sb.WriteString("\nСамые длинные сообщения:\n")
		for i := 0; i < longN; i++ {
			sb.WriteString(fmt.Sprintf("%d. [%d символов]\n   %s\n", i+1, longLines[i].Len, longLines[i].Line))
		}
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

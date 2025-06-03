package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Model — структура состояния приложения
type Model struct {
	histogram map[string]int  // Частота логов по времени
	logLines  []string        // Строки лог-файла
	viewport  viewport.Model  // Для прокрутки логов
	textInput textinput.Model // Для ввода команд
	logFile   string          // Имя лог-файла
	width     int             // Ширина терминала
	height    int             // Высота терминала
	minTime   time.Time       // Самый ранний таймштамп в логах
	maxTime   time.Time       // Самый поздний таймштамп в логах
	err       error           // Ошибки

	filterMode bool   // режим фильтрации
	filterExpr string // последнее выражение фильтра
	gotoMode   bool   // режим перехода по таймштампу

	mainTimestampFormat string // основной формат таймштампа, определённый из первой строки

	analysisResults    map[string]string // результаты этапов анализа
	analysisInProgress bool              // идет ли сейчас анализ
}

func initialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter command"
	ti.Focus()
	ti.Prompt = ""

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

// Разбор таймштампа из строки лога
func parseTimestamp(timestampStr string) (time.Time, error) {
	var t time.Time
	var err error
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
	layoutParts := strings.Split(format, " ")
	inputParts := strings.Split(input, " ")
	for i := range inputParts {
		if len(inputParts[i]) < len(layoutParts[i]) {
			inputParts[i] += layoutParts[i][len(inputParts[i]):]
		}
	}
	result := input
	if len(input) < len(format) {
		result += format[len(input):]
	}
	return result
}

// Загрузка и обработка лог-файла
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

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		var timestamp time.Time
		var err error

		timestampStr := fields[0]
		timestamp, err = parseTimestamp(timestampStr)
		if err != nil && len(fields) >= 2 {
			timestampStr = fields[0] + " " + fields[1]
			timestamp, err = parseTimestamp(timestampStr)
		}
		if err != nil && len(fields) >= 3 {
			timestampStr = fields[0] + " " + fields[1] + " " + fields[2]
			timestamp, err = parseTimestamp(timestampStr)
		}
		if err != nil {
			continue
		}

		if timestamp.Before(minTime) {
			minTime = timestamp
		}
		if timestamp.After(maxTime) {
			maxTime = timestamp
		}

		minute := timestamp.Format("2006-01-02 15:04")
		histogram[minute]++
	}

	if err := scanner.Err(); err != nil {
		return errorMsg{err}
	}

	mainFormat := detectMainTimestampFormat(logLines)

	return logFileLoadedMsg{
		histogram:           histogram,
		logLines:            logLines,
		minTime:             minTime,
		maxTime:             maxTime,
		mainTimestampFormat: mainFormat,
	}
}

// Типы сообщений для tea
type errorMsg struct{ err error }
type logFileLoadedMsg struct {
	histogram           map[string]int
	logLines            []string
	minTime             time.Time
	maxTime             time.Time
	mainTimestampFormat string
}
type analysisStepMsg struct {
	StepName string
	Content  string
}

// Реализация tea.Model — Init
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

// Реализация tea.Model — Update
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.filterMode {
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

				if m.mainTimestampFormat != "" {
					completedInput := completeTimestamp(inputTS, m.mainTimestampFormat)
					target, parseErr = time.Parse(m.mainTimestampFormat, completedInput)
				} else {
					parseErr = fmt.Errorf("не удалось определить формат таймштампа")
				}

				if parseErr != nil {
					m.viewport.SetContent(fmt.Sprintf("Ошибка разбора таймштампа: %v", parseErr))
				} else {
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
				m.analysisResults = map[string]string{
					"patterns":   "Вычисление...",
					"rare":       "Вычисление...",
					"long":       "Вычисление...",
					"suspicious": "Вычисление...",
					"ngrams":     "Вычисление...",
				}
				m.analysisInProgress = true
				m.viewport.SetContent(joinAnalysisResults(m.analysisResults))
				return m, analyseLogAsync(m.logLines)
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

		histogramHeight := 10
		inputHeight := 3

		viewportWidth := m.width - 4
		inputWidth := m.width - 4

		m.viewport.Height = m.height - histogramHeight - inputHeight
		m.viewport.Width = viewportWidth

		m.textInput.Width = inputWidth - 5

	case logFileLoadedMsg:
		m.histogram = msg.histogram
		m.logLines = msg.logLines
		m.minTime = msg.minTime
		m.maxTime = msg.maxTime
		m.mainTimestampFormat = msg.mainTimestampFormat

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

	case analysisStepMsg:
		if m.analysisResults == nil {
			m.analysisResults = make(map[string]string)
		}
		m.analysisResults[msg.StepName] = msg.Content
		m.viewport.SetContent(joinAnalysisResults(m.analysisResults))

		// Проверка завершения всех этапов
		allDone := true
		for _, v := range m.analysisResults {
			if strings.HasPrefix(v, "Вычисление") {
				allDone = false
				break
			}
		}
		if allDone {
			m.analysisInProgress = false
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
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
	for i := 1; i <= 3 && i < len(fields); i++ {
		if _, err := parseTimestamp(strings.Join(fields[:i], " ")); err == nil {
			line = strings.Join(fields[i:], " ")
			break
		}
	}
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

// --- РЕАЛИЗУЙТЕ ЭТАПЫ АНАЛИЗА КАК ОТДЕЛЬНЫЕ ФУНКЦИИ ---
// Можно вынести из buildLogAnalysis соответствующие куски кода:

func analysePatterns(logLines []string) string {
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
	for i := 0; i < topN; i++ {
		sb.WriteString(fmt.Sprintf("%d. [%d раз]\n   Паттерн: %s\n   Пример: %s\n", i+1, stats[i].Count, stats[i].Pattern, stats[i].Example))
	}
	return sb.String()
}

func analyseRarePatterns(logLines []string) string {
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
	var stats []patternStat
	for _, v := range patterns {
		stats = append(stats, *v)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Count < stats[j].Count })
	rareN := 5
	if len(stats) < rareN {
		rareN = len(stats)
	}
	var sb strings.Builder
	for i := 0; i < rareN; i++ {
		sb.WriteString(fmt.Sprintf("%d. [%d раз]\n   Паттерн: %s\n   Пример: %s\n", i+1, stats[i].Count, stats[i].Pattern, stats[i].Example))
	}
	if rareN == 0 {
		sb.WriteString("Нет уникальных или редких паттернов.\n")
	}
	return sb.String()
}

func analyseLongLines(logLines []string) string {
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
	var sb strings.Builder
	for i := 0; i < longN; i++ {
		sb.WriteString(fmt.Sprintf("%d. [%d символов]\n   %s\n", i+1, longLines[i].Len, longLines[i].Line))
	}
	return sb.String()
}

func analyseSuspicious(logLines []string) string {
	type suspiciousPattern struct {
		Label string
		Regex *regexp.Regexp
	}
	suspiciousPatterns := []suspiciousPattern{
		{"fail", regexp.MustCompile(`(?i)\bfail(ed|ing|s)?\b`)},
		{"exception", regexp.MustCompile(`(?i)\bexception(s)?\b`)},
		{"panic", regexp.MustCompile(`(?i)\bpanic(s|ed|ing)?\b`)},
		{"critical", regexp.MustCompile(`(?i)\bcritical\b`)},
		{"abort", regexp.MustCompile(`(?i)\babort(ed|ing|s)?\b`)},
		{"timeout", regexp.MustCompile(`(?i)\btime\s?out(s|ed|ing)?\b`)},
		{"traceback", regexp.MustCompile(`(?i)\btraceback\b`)},
		{"unreachable", regexp.MustCompile(`(?i)\bunreachable\b`)},
		{"unhandled", regexp.MustCompile(`(?i)\bunhandled\b`)},
		{"fatal", regexp.MustCompile(`(?i)\bfatal\b`)},
		{"segfault", regexp.MustCompile(`(?i)\bsegfault\b`)},
		{"stacktrace", regexp.MustCompile(`(?i)\bstack\s?trace\b`)},
		{"not found", regexp.MustCompile(`(?i)not found`)},
		{"could not", regexp.MustCompile(`(?i)could not`)},
		{"no such file", regexp.MustCompile(`(?i)no such file`)},
		{"connection refused", regexp.MustCompile(`(?i)connection refused`)},
		{"permission denied", regexp.MustCompile(`(?i)permission denied`)},
		{"out of memory", regexp.MustCompile(`(?i)out of memory`)},
		{"disk full", regexp.MustCompile(`(?i)disk full`)},
		{"broken pipe", regexp.MustCompile(`(?i)broken pipe`)},
	}
	var sb strings.Builder
	foundAny := false
	for _, pat := range suspiciousPatterns {
		var matches []string
		for i := len(logLines) - 1; i >= 0 && len(matches) < 3; i-- {
			if pat.Regex.MatchString(logLines[i]) {
				matches = append(matches, logLines[i])
			}
		}
		if len(matches) > 0 {
			foundAny = true
			sb.WriteString(fmt.Sprintf("  %s (последние %d):\n", pat.Label, len(matches)))
			for i := len(matches) - 1; i >= 0; i-- {
				sb.WriteString(fmt.Sprintf("    %s\n", matches[i]))
			}
		}
	}
	if !foundAny {
		sb.WriteString("  Не найдено подозрительных сообщений.\n")
	}
	return sb.String()
}

func analyseNgrams(logLines []string) string {
	type ngramStat struct {
		Phrase string
		Count  int
	}
	fourgramFreq := make(map[string]int)
	for _, line := range logLines {
		norm := normalizeLogLine(line)
		words := strings.Fields(norm)
		for i := 0; i < len(words)-3; i++ {
			fourgram := words[i] + " " + words[i+1] + " " + words[i+2] + " " + words[i+3]
			fourgramFreq[fourgram]++
		}
	}
	var fourgramStats []ngramStat
	for k, v := range fourgramFreq {
		fourgramStats = append(fourgramStats, ngramStat{k, v})
	}
	sort.Slice(fourgramStats, func(i, j int) bool { return fourgramStats[i].Count > fourgramStats[j].Count })
	if len(fourgramStats) > 10 {
		fourgramStats = fourgramStats[:10]
	}
	var sb strings.Builder
	for i, stat := range fourgramStats {
		sb.WriteString(fmt.Sprintf("%d. [%d] %s\n", i+1, stat.Count, stat.Phrase))
	}
	return sb.String()
}

// Функция для запуска анализа логов асинхронно
func analyseLogAsync(logLines []string) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return analysisStepMsg{StepName: "patterns", Content: analysePatterns(logLines)}
		},
		func() tea.Msg {
			return analysisStepMsg{StepName: "rare", Content: analyseRarePatterns(logLines)}
		},
		func() tea.Msg {
			return analysisStepMsg{StepName: "long", Content: analyseLongLines(logLines)}
		},
		func() tea.Msg {
			return analysisStepMsg{StepName: "suspicious", Content: analyseSuspicious(logLines)}
		},
		func() tea.Msg {
			return analysisStepMsg{StepName: "ngrams", Content: analyseNgrams(logLines)}
		},
	)
}

// Функция для сборки вывода результатов анализа
func joinAnalysisResults(results map[string]string) string {
	order := []string{"patterns", "rare", "long", "suspicious", "ngrams"}
	titles := map[string]string{
		"patterns":   "Анализ лог-файла: самые частые паттерны сообщений",
		"rare":       "Редкие (уникальные или почти уникальные) паттерны",
		"long":       "Самые длинные сообщения",
		"suspicious": "Подозрительные сообщения по ключевым словам и шаблонам",
		"ngrams":     "Топ-10 четырёхграмм (четырёхсловных фраз)",
	}
	var sb strings.Builder
	for _, k := range order {
		if v, ok := results[k]; ok {
			sb.WriteString("\n" + titles[k] + ":\n")
			sb.WriteString(v)
			sb.WriteString("\n")
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

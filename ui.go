package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View отвечает за отрисовку интерфейса
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Ошибка: %v\n\nНажмите любую клавишу для выхода.", m.err)
	}

	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(0, 1)

	histogramView := m.renderHistogram()
	histogramBox := borderStyle.Width(m.width - borderStyle.GetHorizontalFrameSize() + 2).Render(histogramView)

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

	logOutputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD"))
	logOutput := logOutputStyle.Width(m.width - logOutputStyle.GetHorizontalFrameSize()).Render(m.viewport.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		histogramBox,
		commandInput,
		logOutput,
	)
}

// Визуализация гистограммы
func (m Model) renderHistogram() string {
	if len(m.histogram) == 0 {
		return "Загрузка гистограммы..."
	}

	histWidth := m.width - 4 // 2 символа на каждую сторону рамки

	var times []string
	for t := range m.histogram {
		times = append(times, t)
	}
	sort.Strings(times)
	if len(times) == 0 || histWidth < 2 {
		return "Недостаточно данных для гистограммы"
	}

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

	startTime := timePoints[0]
	endTime := timePoints[len(timePoints)-1]
	totalDuration := endTime.Sub(startTime)
	if totalDuration == 0 {
		totalDuration = time.Minute
	}

	binCounts := make([]int, histWidth)
	binStarts := make([]time.Time, histWidth)
	binDuration := totalDuration / time.Duration(histWidth)

	for i := 0; i < histWidth; i++ {
		binStarts[i] = startTime.Add(time.Duration(i) * binDuration)
	}

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

	startLabel := startTime.Format("2006-01-02 15:04")
	midLabel := startTime.Add(totalDuration / 2).Format("2006-01-02 15:04")
	endLabel := endTime.Format("2006-01-02 15:04")

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

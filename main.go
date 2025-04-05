package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Список поддерживаемых форматов таймштампов
var timestampFormats = []string{
	"2006/01/02 15:04:05.000000",    // news-notifier.log и udf.log
	"2006-01-02 15:04:05,000000000", // papertrading.log
	// Здесь можно добавить другие форматы при необходимости
}

func parseTimestamp(timestampStr string) (time.Time, error) {
	var t time.Time
	var err error
	for _, format := range timestampFormats {
		t, err = time.Parse(format, timestampStr)
		if err == nil {
			return t, nil
		}
	}
	return t, fmt.Errorf("неизвестный формат таймштампа: %s", timestampStr)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Использование: go run log_histogram.go <имя_лог_файла>")
		return
	}

	logFileName := os.Args[1]

	file, err := os.Open(logFileName)
	if err != nil {
		fmt.Println("Ошибка открытия файла:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	histogram := make(map[string]int)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		timestampStr := fields[0] + " " + fields[1]
		timestamp, err := parseTimestamp(timestampStr)
		if err != nil {
			// Если не удалось распарсить таймштамп, пропускаем строку
			continue
		}

		minute := timestamp.Format("2006-01-02 15:04")
		histogram[minute]++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Ошибка чтения файла:", err)
		return
	}

	fmt.Println("Гистограмма частоты лог-сообщений по минутам:")
	for minute, count := range histogram {
		fmt.Printf("%s | %s (%d)\n", minute, strings.Repeat("*", count), count)
	}
}

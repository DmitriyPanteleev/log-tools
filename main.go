package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func parseTimestamp(timestampStr string) (time.Time, error) {
	var t time.Time
	var err error
	// Список поддерживаемых форматов из файла formats.go
	for _, format := range TimestampFormats {
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
		if len(fields) < 1 {
			continue
		}

		// Попробуем разные варианты форматирования таймштампа
		var timestamp time.Time
		var err error

		// Сначала пробуем только первое поле (для логов типа vector_logger)
		timestampStr := fields[0]
		timestamp, err = parseTimestamp(timestampStr)

		// Если не удалось, пробуем первые два поля (традиционный формат)
		if err != nil && len(fields) >= 2 {
			timestampStr = fields[0] + " " + fields[1]
			timestamp, err = parseTimestamp(timestampStr)
		}

		// Если не удалось, пробуем первые три поля
		if err != nil && len(fields) >= 3 {
			timestampStr = fields[0] + " " + fields[1] + " " + fields[2]
			timestamp, err = parseTimestamp(timestampStr)
		}

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

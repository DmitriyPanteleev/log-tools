package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

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
		timestamp, err := time.Parse("2006/01/02 15:04:05.000000", timestampStr)
		if err != nil {
			fmt.Println("Ошибка парсинга времени:", err)
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

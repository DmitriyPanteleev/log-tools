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
	"2006/01/02 15:04:05.000000",          // news-notifier.log и udf.log
	"2006-01-02 15:04:05,000000000",       // papertrading.log
	"2006-01-02 15:04:05.000000",          // generic, dex_metrics, misc_snapshots, etc.
	"2006/01/02 15:04:05.000",             // dashboard, charts-storage, scanner, doge
	"2006-01-02 15:04:05,000",             // kafka, runner
	"2006-01-02T15:04:05.000000",          // pine_facade, tvfs logs
	"2006-01-02T15:04:05.000",             // java_tvbs, runner
	"2006-01-02 15:04:05",                 // basic format without fractional seconds
	"2006-01-02T15:04:05Z",                // ISO format with Z
	"2006-01-02T15:04:05.000Z",            // vector_logger, envoy-proxy
	"2006-01-02T15:04:05.000000Z",         // vector_logger with microseconds
	"2006.01.02 15:04:05.000",             // clickhouse
	"Jan 2 15:04:05.000 2006",             // query_log, manticore_query
	"02/Jan/2006 15:04:05.000",            // gunicorn, saveload
	"02/Jan/2006:15:04:05 -0700",          // nginx logs format
	"Mon Jan 2 15:04:05 2006",             // get_index
	"2 Jan 2006 15:04:05",                 // etcd
	"2006-01-02 15:04:05.000+0000",        // with explicit timezone
	"2006-01-02 15:04:05-0700",            // with timezone offset
	"2006-01-02T15:04:05-07:00",           // RFC3339 format
	"2006-01-02T15:04:05.999999999Z07:00", // RFC3339 with nanoseconds
	"Mon, 02 Jan 2006 15:04:05 MST",       // RFC1123
	"Mon, 02 Jan 2006 15:04:05 -0700",     // RFC1123Z
	"02 Jan 06 15:04 MST",                 // RFC822
	"02 Jan 06 15:04 -0700",               // RFC822Z
	"Monday, 02-Jan-06 15:04:05 MST",      // RFC850
	"Mon Jan _2 15:04:05 2006",            // ANSIC
	"Mon Jan _2 15:04:05 MST 2006",        // UnixDate
	"Mon Jan 02 15:04:05 -0700 2006",      // RubyDate
	"Jan _2 15:04:05",                     // Stamp format
	"Jan _2 15:04:05.000",                 // StampMilli
	"Jan _2 15:04:05.000000",              // StampMicro
	"Jan _2 15:04:05.000000000",           // StampNano
	"Jan 02 15:04:05",                     // Common syslog format
	"2006-01-02 15:04:05.999999-07",       // PostgreSQL timestamp format
	"20060102150405",                      // Compact timestamp without separators
	"20060102150405-0700",                 // Compact timestamp with timezone
	"2006-01-02 15:04:05.999999999 -0700", // Full timestamp with timezone and nanoseconds
	"Jan 2 15:04:05",                      // rsyslogd format
	"Jan _2 15:04:05",                     // rsyslogd format with padding
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

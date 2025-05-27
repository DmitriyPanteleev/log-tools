package main

// TimestampFormats здесь перечислены форматы таймштампов, которые могут использоваться в логах.
var TimestampFormats = []string{
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

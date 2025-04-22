package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func parseMarkdownTable(input string) (headers []string, values [][]string, err error) {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	if len(lines) < 2 {
		return nil, nil, errors.New("nicht genug Zeilen für eine gültige Markdown-Tabelle")
	}
	// parse header row parsen
	headers = parseMarkdownRow(lines[0])
	// ignore separator line (e.g. |----|----|...)
	for i := 2; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		row := parseMarkdownRow(line)
		if len(row) != len(headers) {
			return nil, nil, fmt.Errorf("line %d has %d colums, expected %d", i+1, len(row), len(headers))
		}
		values = append(values, row)
	}
	return headers, values, nil
}

func parseMarkdownRow(line string) []string {
	// remove leading and trailing "|"
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	var cells []string
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func loadFile(templateFile string) string {
	data, err := os.ReadFile(templateFile)
	must(err)
	return string(data)
}

const timeFormat = "2006-01-02T15:04:05.000000000Z07:00"

func writeTempFlux(start, stop time.Time, tempFile string, template string, headers []string, values []string) {
	if len(headers) != len(values) {
		panic("value headers count doesn't match values count")
	}
	flux := strings.ReplaceAll(template, "{{START}}", start.Format(timeFormat))
	flux = strings.ReplaceAll(flux, "{{STOP}}", stop.Format(timeFormat))
	for i, header := range headers {
		flux = strings.ReplaceAll(flux, "{{"+header+"}}", values[i])
	}
	err := os.WriteFile(tempFile, []byte(flux), 0644)
	must(err)
}

func runFluxFile(file string) error {
	cmd := exec.Command("influx", "query", "--file", file)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nil // Ignoriere stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(stderr.String())
	}
	return nil
}

func parseInterval(interval string) (int, string) {
	suffix := interval[len(interval)-1:]
	number := parseInt(interval[:len(interval)-1])
	return number, suffix
}

func parseInt(s string) int {
	val, err := fmt.Sscanf(s, "%d", new(int))
	if err != nil || val == 0 {
		panic("invalid interval format")
	}
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

type cursorIncrementer struct {
	mode string
	num  int
}

func newCursorIncrementer(interval string) *cursorIncrementer {
	num, mode := parseInterval(interval)
	ci := &cursorIncrementer{
		mode: mode,
		num:  num,
	}
	return ci
}

func (ci *cursorIncrementer) IsValid() error {
	_, err := ci.Inc(time.Now())
	return err
}

func (ci *cursorIncrementer) Inc(cursor time.Time) (next time.Time, err error) {
	switch ci.mode {
	case "d":
		next = cursor.AddDate(0, 0, ci.num)
	case "w":
		next = cursor.AddDate(0, 0, ci.num*7)
	case "m":
		next = cursor.AddDate(0, ci.num, 0)
	case "y":
		next = cursor.AddDate(ci.num, 0, 0)
	case "h":
		duration, err := time.ParseDuration(fmt.Sprintf("%dh", ci.num))
		if err != nil {
			return next, errors.New("invalid interval format")
		}
		next = cursor.Add(duration)
	}
	if next.IsZero() || !cursor.Before(next) {
		return next, errors.New("invalid increment")
	}
	return next, nil
}

func printProgressBar(batch, current, total int, start, stop time.Time, running time.Duration) {
	barLength := 50
	percent := float64(current) / float64(total)
	filled := int(percent * float64(barLength))
	if filled == 0 {
		filled = 1
	}

	bar := strings.Repeat("=", filled-1) + ">" + strings.Repeat(" ", barLength-filled)
	fmt.Printf(
		"\rrow %d: Processing: %s → %s [%s] %3.0f%% (🕓 %v)",
		batch,
		start.Format(time.RFC3339), stop.Format(time.RFC3339),
		bar, percent*100, running.Truncate(time.Second).String(),
	)
}

// askYesNo displays a yes/no prompt and returns true for yes, false for no
func askYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s (yes/no): ", prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			os.Exit(1)
		}

		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "yes", "y":
			return true
		case "no", "n":
			return false
		default:
			return false
		}
	}
}

func processLine(start time.Time, end time.Time, ci *cursorIncrementer, tempFileName string, template string, line int, headers []string, values []string) {
	startTime := time.Now()
	cursor := start
	batches := []time.Time{cursor}
	for cursor.Before(end) {
		next, _ := ci.Inc(cursor)
		if next.After(end) {
			next = end
		}
		batches = append(batches, next)
		cursor = next
	}

	// check if temporary file already exists
	if _, err := os.Stat(tempFileName); err == nil {
		// file exists, ask to delete
		if delFile := askYesNo(fmt.Sprintf("Temporary file %q already exists. Delete file to coninue?", tempFileName)); delFile {
			if err := os.Remove(tempFileName); err != nil {
				fmt.Fprintf(os.Stderr, "Error while deleting file: %s\n", err)
				os.Exit(1)
			}
		} else {
			os.Exit(1)
		}
	}

	defer os.Remove(tempFileName)

	for i := 0; i < len(batches)-1; i++ {
		start := batches[i]
		stop := batches[i+1]

		// fmt.Printf("🕓 Processing: %s → %s\n", current.Format(time.RFC3339), next.Format(time.RFC3339))
		printProgressBar(line, i+1, len(batches)-1, start, stop, time.Since(startTime))
		writeTempFlux(start, stop, tempFileName, template, headers, values)
		err := runFluxFile(tempFileName)
		if err != nil {
			fmt.Printf(" ❌ %s\n", err)
			return
		}
	}
	printProgressBar(line, 100, 100, start, end, time.Since(startTime))
	fmt.Print(" ✅\n")
}

func main() {
	startStr := flag.String("start", "", "Start time (RFC3339, e.g. 2024-01-01T00:00:00Z)")
	stopStr := flag.String("stop", "", "Stop time (RFC3339, exclusive)")
	interval := flag.String("interval", "2d", "Interval (e.g. 1d, 1w, 1m, 1y, 48h)")
	templateFile := flag.String("template", "template.flux", "Flux template file")
	valueFile := flag.String("table", "table.md", "Markdown table with replacement values")

	flag.Parse()

	if *startStr == "" || *stopStr == "" {
		fmt.Println("❗ Required arguments are at least --start and --stop")
		os.Exit(1)
	}

	ci := newCursorIncrementer(*interval)
	if err := ci.IsValid(); err != nil {
		fmt.Printf("❗ Invalid interval: %s\n", err)
		os.Exit(1)
	}

	start, err := time.Parse(time.RFC3339, *startStr)
	must(err)
	end, err := time.Parse(time.RFC3339, *stopStr)
	must(err)

	template := loadFile(*templateFile)
	if template == "" {
		fmt.Printf("❗ Template is empty: %s\n", *templateFile)
		os.Exit(1)
	}
	headers, values, err := parseMarkdownTable(loadFile(*valueFile))
	if err != nil {
		fmt.Printf("❗ Invalid template values: %s\n", err)
		os.Exit(1)
	}

	for i, valueLine := range values {
		processLine(start, end, ci, *templateFile+".fluxbatcher.tmp", template, i, headers, valueLine)
	}
	must(err)

}

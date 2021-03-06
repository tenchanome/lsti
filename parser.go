package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ParseMessageFiles parses LS-DYNA message files (e.g. messag, mes****) and return records.
func (cli *CLI) ParseMessageFiles(files []string) ([]*Record, error) {
	sort.Strings(files)
	var records []*Record
	for _, file := range files {
		record, err := cli.ParseMessageFile(file)
		if err != nil {
			fmt.Fprintln(cli.errStream, err)
		}
		records = append(records, record)
	}
	return records, nil
}

// ParseMessageFile parses LS-DYNA message file (e.g. messag, mes****) and return record.
func (cli *CLI) ParseMessageFile(file string) (*Record, error) {
	fp, err := os.Open(filepath.FromSlash(file))
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	// Translate file path.
	abs, err := filepath.Abs(file)
	if err == nil {
		if opts.Out.Abs {
			file = abs
		} else if opts.Out.Relative != "" {
			rel, err := filepath.Rel(filepath.FromSlash(opts.Out.Relative), abs)
			if err == nil {
				file = rel
			}
		}
	}

	record := Record{File: file}
	scanner := bufio.NewScanner(fp)
	start := false
	end := false
	count := 0
	const (
		SMP = "smp"
		MPP = "mpp"
	)
	var currentParent *Parent
	var moduleType string
	for scanner.Scan() {
		line := scanner.Text()

		// Search for header information.
		if !start {
			if strings.Contains(line, "Version : ") {
				record.Version = parseText([]rune(line), 18, 34)
				record.Date = parseText([]rune(line), 34, 55)
				if strings.Contains(record.Version, "smp") {
					moduleType = SMP
				} else if strings.Contains(record.Version, "mpp") {
					moduleType = MPP
				}
				continue
			}
			if strings.Contains(line, "Revision: ") {
				record.Revision, _ = parseInt([]rune(line), 18, 34)
				record.Time = parseText([]rune(line), 34, 55)
				continue
			}
			if strings.Contains(line, "Licensed to: ") {
				record.LicensedTo = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "Issued by  : ") {
				record.IssuedBy = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "Platform   : ") {
				record.Platform = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "OS Level   : ") {
				record.Os = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "Compiler   : ") {
				record.Compiler = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "Hostname   : ") {
				record.Hostname = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "Precision  : ") {
				record.Precision = parseText([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "SVN Version: ") {
				record.SvnVersion, _ = parseInt([]rune(line), 21, 55)
				continue
			}
			if strings.Contains(line, "Input file: ") {
				record.InputFile = parseText([]rune(line), 13, 84)
				continue
			}
			if moduleType == MPP && strings.HasPrefix(line, " MPP execution with") {
				record.NumCpus, _ = parseInt([]rune(line), 19, 27)
				continue
			}
		}

		// Search for timing information block.
		if strings.HasPrefix(line, " T i m i n g   i n f o r m a t i o n") {
			start = true
			continue
		}
		if !start {
			continue
		}

		// Skip 2 header lines.
		count++
		if count <= 2 {
			continue
		}

		// If timing information block ends, stop reading.
		if strings.Contains(line, "-----------------------") {
			end = true
			continue
		}

		// Parse timing information.
		if start && !end {
			isParent := !strings.HasPrefix(line, "    ")
			runes := []rune(line)
			name := parseName(runes, 0, 25)
			cpuSec, _ := parseFloat(runes, 25, 36)
			cpuPercent, _ := parseFloat(runes, 36, 44)
			clockSec, _ := parseFloat(runes, 44, 58)
			clockPercent, _ := parseFloat(runes, 58, 66)
			if isParent {
				// Parent
				currentParent = record.AddParent(name, cpuSec, cpuPercent, clockSec, clockPercent)
			} else {
				// Child
				currentParent.AddChild(name, cpuSec, cpuPercent, clockSec, clockPercent)
			}
		}

		// Search for footer information.
		if end {
			if moduleType == SMP && strings.HasPrefix(line, " Number of CPU's") {
				record.NumCpus, _ = parseInt([]rune(line), 16, 21)
				continue
			}
			if strings.HasPrefix(line, " N o r m a l    t e r m i n a t i o n") {
				record.NormalTermination = true
				continue
			}
			if strings.HasPrefix(line, " Elapsed time") {
				// Use regexp because Elapsed time is not a fixed format.
				r := regexp.MustCompile(`^ Elapsed time\s*(\d+)\s*seconds`)
				results := r.FindStringSubmatch(line)
				if len(results) == 2 {
					seconds, _ := strconv.ParseFloat(results[1], 64)
					record.ElapsedTime = seconds
				}
				continue
			}
		}
	}
	return &record, nil
}

func parseName(runes []rune, start, end int) string {
	str := string(runes[start:end])
	return strings.TrimRight(strings.TrimRight(strings.Trim(str, " "), "."), " ")
}

func parseText(runes []rune, start, end int) string {
	str := string(runes[start:end])
	return strings.Trim(str, " ")
}

func parseInt(runes []rune, start, end int) (int64, error) {
	str := string(runes[start:end])
	str = strings.Trim(str, " ")
	return strconv.ParseInt(str, 10, 64)
}

func parseFloat(runes []rune, start, end int) (float64, error) {
	str := string(runes[start:end])
	str = strings.Trim(str, " ")
	return strconv.ParseFloat(str, 64)
}

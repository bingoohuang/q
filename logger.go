// Copyright 2016 Ryan Boehning. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package q

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type color string

const (
	// ANSI color escape codes.
	bold     color = "\033[1m"
	yellow   color = "\033[33m"
	cyan     color = "\033[36m"
	endColor color = "\033[0m" // "reset everything"

	maxLineWidth = 80
)

// logger writes pretty logs to the $TMPDIR/$USER.q file. It takes care of opening and
// closing the file. It is safe for concurrent use.
type logger struct {
	start     time.Time    // time of first write in the current log group
	lastWrite time.Time    // last time buffer was flushed. determines when to print header
	lastFile  string       // last file to call q.Q(). determines when to print header
	lastFunc  string       // last function to call q.Q(). determines when to print header
	buf       bytes.Buffer // collects writes before they're flushed to the log file
	mu        sync.Mutex   // protects all the other fields
}

// header returns a formatted header string, e.g. [14:00:36 main.go main.main:122]
// if the 2s timer has expired, or the calling function or filename has changed.
// If none of those things are true, it returns an empty string.
func (l *logger) header(funcName, file string, line int) string {
	if !l.shouldPrintHeader(funcName, file) {
		return ""
	}

	now := time.Now()
	l.start = now
	l.lastFunc = funcName
	l.lastFile = file

	return fmt.Sprintf("[%s %s:%d %s]\n[PID: %d os.Args: %s]",
		now.Format("2006-01-02T15:04:05.000"),
		shortFile(file), line, funcName,
		os.Getpid(), QuoteCommand(os.Args))
}

var pattern = regexp.MustCompile(`[^\w@%+=:,./-]`)

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func Quote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	if pattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	return s
}

// QuoteCommand returns a shell-escaped version of the slice of strings.
// The returned value is a string that can safely be used as shell command arguments.
func QuoteCommand(args []string) string {
	l := make([]string, len(args))

	for i, s := range args {
		l[i] = Quote(s)
	}

	return strings.Join(l, " ")
}
func (l *logger) shouldPrintHeader(funcName, file string) bool {
	if file != l.lastFile {
		return true
	}

	if funcName != l.lastFunc {
		return true
	}

	// If less than 2s has elapsed, this log line will be printed under the
	// previous header.
	const timeWindow = 2 * time.Second

	return time.Since(l.lastWrite) > timeWindow
}

var path = func() string {
	uid := os.Getuid()
	str := strconv.Itoa(uid)
	if u, _ := user.LookupId(str); u != nil {
		return filepath.Join(os.TempDir(), "q."+u.Username)
	}

	return filepath.Join(os.TempDir(), "q")
}()

// flush writes the logger's buffer to disk.
func (l *logger) flush() (err error) {
	err = AppendFile(path, l.buf.Bytes(), 0o666)
	l.lastWrite = time.Now()
	l.buf.Reset()
	if err != nil {
		return fmt.Errorf("failed to flush q buffer: %w", err)
	}

	return nil
}

// AppendFile appends data to a file.
func AppendFile(name string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", name, err)
	}
	err1 := os.Chmod(name, mode)
	if err1 != nil {
		err1 = fmt.Errorf("chmod %s to mod %s: %w", name, mode, err)
	}
	_, err2 := f.Write(data)
	if err2 != nil {
		err2 = fmt.Errorf("write %s: %w", name, err2)
	}
	err3 := f.Close()
	if err3 != nil {
		err3 = fmt.Errorf("close %s: %w", name, err3)
	}

	return MergeErrors(err1, err2, err3)
}

// MultiError represents an error that aggregates multiple errors.
type MultiError []error

// Error implements the error interface for MultiError.
func (m MultiError) Error() string {
	var errorStrings []string
	for _, err := range m {
		errorStrings = append(errorStrings, err.Error())
	}
	return strings.Join(errorStrings, "; ")
}

// MergeErrors merges multiple errors into a single error.
func MergeErrors(errors ...error) error {
	var nonNilErrors []error

	for _, err := range errors {
		if err != nil {
			nonNilErrors = append(nonNilErrors, err)
		}
	}

	if len(nonNilErrors) == 0 {
		return nil
	}

	return MultiError(nonNilErrors)
}

// output writes to the log buffer. Each log message is prepended with a
// timestamp. Long lines are broken at 80 characters.
func (l *logger) output(args ...string) {
	timestamp := fmt.Sprintf("%.3fs", time.Since(l.start).Seconds())
	timestampWidth := len(timestamp) + 1 // +1 for padding space after timestamp
	timestamp = colorize(timestamp, yellow)

	// preWidth is the length of everything before the log message.
	fmt.Fprint(&l.buf, timestamp, " ")

	// Subsequent lines have to be indented by the width of the timestamp.
	indent := strings.Repeat(" ", timestampWidth)
	padding := "" // padding is the space between args.
	lineArgs := 0 // number of args printed on the current log line.
	lineWidth := timestampWidth
	for _, arg := range args {
		argWidth := argWidth(arg)
		lineWidth += argWidth + len(padding)

		// Some names in name=value strings contain newlines. Insert indentation
		// after each newline so they line up.
		arg = strings.ReplaceAll(arg, "\n", "\n"+indent)

		// Break up long lines. If this is first arg printed on the line
		// (lineArgs == 0), it makes no sense to break up the line.
		if lineWidth > maxLineWidth && lineArgs != 0 {
			fmt.Fprint(&l.buf, "\n", indent)
			lineArgs = 0
			lineWidth = timestampWidth + argWidth
			padding = ""
		}
		fmt.Fprint(&l.buf, padding, arg)
		lineArgs++
		padding = " "
	}

	fmt.Fprint(&l.buf, "\n")
}

// shortFile takes an absolute file path and returns just the <directory>/<file>,
// e.g. "foo/bar.go".
func shortFile(file string) string {
	dir := filepath.Base(filepath.Dir(file))
	file = filepath.Base(file)

	return filepath.Join(dir, file)
}

package services

import (
	"bufio"
	"io"
)

// maxStreamLine bounds a single log record held in memory while streaming.
// bufio.Scanner's default cap (64 KiB) made any longer line abort the pipe
// reader, which left the child process blocked on stdout/stderr write() and
// the whole deploy pipeline hanging until its 25-minute context deadline.
// 256 KiB covers base64 blobs / minified build output while still capping
// per-line allocations.
const maxStreamLine = 256 << 10

// streamLines emits newline- or carriage-return-delimited records from r without
// bufio.Scanner's 64 KiB hard limit. Records longer than maxStreamLine are emitted once,
// truncated, with an explicit marker suffix; the reader always keeps
// consuming until EOF so a producer blocked on write() can never wedge the
// pipeline. Unlike bufio.Scanner this never reports ErrTooLong.
func streamLines(r io.Reader, emit func(string)) error {
	reader := bufio.NewReader(r)
	var (
		buf       []byte
		truncated bool
		sawCR     bool
	)

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if len(buf) > 0 || truncated {
				line := string(buf)
				if truncated {
					line += " ...[truncated line]"
				}
				emit(line)
			}
			if err == io.EOF {
				return nil
			}
			return err
		}

		if b == '\r' {
			line := string(buf)
			if truncated {
				line += " ...[truncated line]"
			}
			emit(line)
			buf = buf[:0]
			truncated = false
			sawCR = true
			continue
		}

		if b == '\n' {
			if sawCR {
				sawCR = false
				continue
			}
			line := string(buf)
			if truncated {
				line += " ...[truncated line]"
			}
			emit(line)
			buf = buf[:0]
			truncated = false
			continue
		}

		sawCR = false
		if len(buf) < maxStreamLine {
			buf = append(buf, b)
		} else {
			truncated = true
		}
	}
}

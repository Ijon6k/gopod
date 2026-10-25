package services

import (
	"bufio"
	"io"
	"strings"
)

// maxStreamLine bounds a single log record held in memory while streaming.
// bufio.Scanner's default cap (64 KiB) made any longer line abort the pipe
// reader, which left the child process blocked on stdout/stderr write() and
// the whole deploy pipeline hanging until its 25-minute context deadline.
// 256 KiB covers base64 blobs / minified build output while still capping
// per-line allocations.
const maxStreamLine = 256 << 10

// streamLines emits newline-delimited records from r without the
// bufio.Scanner's 64 KiB hard limit. Records longer than maxStreamLine are emitted once,
// truncated, with an explicit marker suffix; the reader always keeps
// consuming until EOF so a producer blocked on write() can never wedge the
// pipeline. Unlike bufio.Scanner this never reports ErrTooLong.
func streamLines(r io.Reader, emit func(string)) error {
	reader := bufio.NewReader(r)
	var (
		buf       []byte
		truncated bool
	)

	slice, err := reader.ReadSlice('\n')
	for {
		switch {
		case err == bufio.ErrBufferFull:
			// Partial record: retain at most maxStreamLine bytes and drop
			// the rest; remember the truncation for the final emit.
			if room := maxStreamLine - len(buf); room > 0 {
				if room > len(slice) {
					room = len(slice)
				}
				buf = append(buf, slice[:room]...)
			}
			if len(buf) >= maxStreamLine {
				truncated = true
			}
			slice, err = reader.ReadSlice('\n')
			continue

		case err == nil || err == io.EOF:
			// True EOF with nothing buffered: no record to emit (otherwise a
			// file ending in "\n" would gain one bogus empty record).
			if err == io.EOF && !truncated && len(buf) == 0 && len(slice) == 0 {
				return nil
			}
			switch {
			case truncated:
				emit(string(buf) + " ...[truncated line]")
			default:
				emit(strings.TrimRight(string(buf)+string(slice), "\n"))
			}
			buf = buf[:0]
			truncated = false
			if err == io.EOF {
				return nil
			}
			slice, err = reader.ReadSlice('\n')

		default:
			return err
		}
	}
}

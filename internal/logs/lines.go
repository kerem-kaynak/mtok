package logs

import (
	"bufio"
	"io"
)

// forEachLine streams newline-delimited records to fn without loading the
// file into memory. Session logs can contain multi-megabyte lines (embedded
// tool output, images), so lines longer than the buffer are accumulated
// rather than dropped. The slice passed to fn is only valid for the call.
func forEachLine(r io.Reader, fn func(line []byte)) error {
	br := bufio.NewReaderSize(r, 1<<20)
	var overflow []byte
	for {
		chunk, err := br.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			overflow = append(overflow, chunk...)
			continue
		}
		if len(overflow) > 0 {
			overflow = append(overflow, chunk...)
			fn(overflow)
			overflow = overflow[:0]
		} else if len(chunk) > 0 {
			fn(chunk)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

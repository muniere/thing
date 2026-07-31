package daemon

import (
	"bufio"
	"io"
	"os"
	"time"
)

// ReadLastLines returns the last n lines of the file at path. n <= 0 returns all
// lines. A missing file yields nil with ok=false so callers can distinguish "no
// log yet" from an empty log. The file is read whole; the daemon log is small
// enough that a streaming ring buffer isn't worth the complexity.
func ReadLastLines(path string, n int) (lines []string, ok bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, true, err
	}
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, true, nil
}

// Follow streams appended content of the file at path to w, starting from the
// end, until stop is closed. It polls for growth; a truncation (the file
// shrinking, e.g. after a fresh start) rewinds to the new end. It blocks until
// stop fires, so callers run it against an interrupt signal.
func Follow(path string, w io.Writer, stop <-chan struct{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	buf := make([]byte, 32*1024)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-tick.C:
		}
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if fi.Size() < offset { // truncated: follow the new file from its end.
			offset = fi.Size()
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return err
			}
		}
		for {
			nr, err := f.Read(buf)
			if nr > 0 {
				offset += int64(nr)
				if _, werr := w.Write(buf[:nr]); werr != nil {
					return werr
				}
			}
			if err == io.EOF || nr == 0 {
				break
			}
			if err != nil {
				return err
			}
		}
	}
}

package providers

import (
	"bufio"
	"bytes"
	"io"
)

// readEvents walks a server-sent-event stream and hands each event's name and
// joined data to fn; fn returning an error ends the read. Comments and
// unknown fields are skipped; a "data: [DONE]" sentinel ends it cleanly.
// The Anthropic and Responses clients stream through this (the OpenAI-
// shaped client has its own accumulator in oai_stream.go).
func readEvents(r io.Reader, fn func(event string, data []byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var event string
	var data [][]byte
	flush := func() error {
		if len(data) == 0 {
			event = ""
			return nil
		}
		joined := bytes.Join(data, []byte("\n"))
		ev := event
		data, event = nil, ""
		if bytes.Equal(bytes.TrimSpace(joined), []byte("[DONE]")) {
			return io.EOF
		}
		return fn(ev, joined)
	}
	for scanner.Scan() {
		line := scanner.Bytes()
		switch {
		case len(bytes.TrimSpace(line)) == 0:
			if err := flush(); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		case bytes.HasPrefix(line, []byte("event:")):
			event = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
		case bytes.HasPrefix(line, []byte("data:")):
			d := bytes.TrimPrefix(line, []byte("data:"))
			if len(d) > 0 && d[0] == ' ' {
				d = d[1:]
			}
			data = append(data, append([]byte(nil), d...))
		}
	}
	if err := flush(); err != nil && err != io.EOF {
		return err
	}
	return scanner.Err()
}

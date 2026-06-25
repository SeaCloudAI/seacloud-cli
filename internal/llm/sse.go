package llm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func parseSSE(r io.Reader, protocol string, opts StreamOptions) (*Result, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	state := &streamState{protocol: protocol}
	var event string
	var dataLines []string

	dispatch := func() error {
		if len(dataLines) == 0 {
			event = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		defer func() { event = "" }()
		return state.dispatch(event, data, opts)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if opts.Raw != nil {
			if _, err := fmt.Fprintln(opts.Raw, line); err != nil {
				return nil, err
			}
		}
		if line == "" {
			if err := dispatch(); err != nil {
				return nil, err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(dataLines) > 0 {
		if err := dispatch(); err != nil {
			return nil, err
		}
	}
	if !state.completed {
		return nil, fmt.Errorf("stream ended before completion marker")
	}
	return state.finish(), nil
}

package parsers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/bluenviron/gomavlib/v3/pkg/dialect"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/all"
	"github.com/bluenviron/gomavlib/v3/pkg/frame"
)

const (
	MavLink = "mavlink"
)

// MavlinkParser maintains stream state for MAVLink parsing
type MavlinkParser struct {
	reader *frame.Reader
}

// NewMavlinkParser initializes a MAVLink stream parser
func NewMavlinkParser(buf *bytes.Buffer) (*MavlinkParser, error) {
	dialectRW, err := dialect.NewReadWriter(all.Dialect)
	if err != nil {
		return nil, fmt.Errorf("create dialect: %w", err)
	}
	// Create a reader from the buffered data
	reader, err := frame.NewReader(frame.ReaderConf{
		Reader:    buf,
		DialectRW: dialectRW,
	})
	return &MavlinkParser{
		reader: reader,
	}, nil
}

// Parse processes incoming MAVLink bytes and returns JSON-encoded messages
func (p *MavlinkParser) Parse() []byte {

	var messages []interface{}
	// Read all complete frames from buffer
	for {
		f, err := p.reader.Read()
		if err != nil {
			if err == io.EOF {
				// No more complete frames, preserve remaining bytes
				break
			}
			// Other errors might indicate corrupt data
			return nil
		}
		messages = append(messages, f)
		// msg := f.GetMessage()
		// messages = append(messages, msg)
	}

	// If no messages parsed, return empty
	if len(messages) == 0 {
		return nil
	}

	// Marshal all messages as JSON array or single object
	result, err := json.Marshal(messages)
	if err != nil {
		return nil
	}

	return result
}

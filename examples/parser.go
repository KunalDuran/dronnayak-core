package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/KunalDuran/dronnayak-core/internal/parsers"
)

func main() {

	buf := bytes.NewBuffer(make([]byte, 0, 1024))

	parser, _ := parsers.NewMavlinkParser(buf)
	file, err := os.Open("messages_mav.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Split on ASCII column (|)
		parts := strings.Split(line, "|")
		if len(parts) == 0 {
			continue
		}

		// Remove offset, keep hex bytes
		fields := strings.Fields(parts[0])
		if len(fields) < 2 {
			continue
		}

		hexBytes := fields[1:] // skip offset

		buffer := make([]byte, 0, 1024)
		for _, hb := range hexBytes {
			b, err := hex.DecodeString(hb)
			if err != nil {
				continue
			}
			buffer = append(buffer, b...)
		}
		buf.Write(buffer)

		data := parser.Parse()
		if len(data) == 0 {
			continue
		}
		fmt.Println(string(data))
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

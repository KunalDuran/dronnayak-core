package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// FileWriterWorker appends every received message to filePath.
// Returns the WorkerFunc, a teardown that closes the file, and any setup error.
//
// Add new worker functions here and wire them into the switch in manageWorker.
func FileWriterWorker(filePath string) (WorkerFunc, func(), error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, nil, fmt.Errorf("create output dir: %w", err)
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open output file: %w", err)
	}
	slog.Info("file writer opened", "path", filePath)

	fn := func(_ context.Context, data []byte) error {
		_, err := f.Write(data)
		return err
	}
	teardown := func() {
		f.Close()
		slog.Info("file writer closed", "path", filePath)
	}
	return fn, teardown, nil
}

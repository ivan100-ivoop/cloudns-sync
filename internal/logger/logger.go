package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Logger struct {
	errorLog *log.Logger
	file     *os.File
}

func New(directory, errorFile string) (*Logger, error) {
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	name := strings.ReplaceAll(errorFile, "{date}", time.Now().Format("2006-01-02"))
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open error log: %w", err)
	}
	return &Logger{errorLog: log.New(file, "", log.LstdFlags), file: file}, nil
}

func (logger *Logger) Error(operation, backend, path, zone string, err error) {
	logger.errorLog.Printf("operation=%s source=%s path=%s zone=%s error=%s", operation, backend, path, zone, Sanitize(err.Error()))
}

func (logger *Logger) Close() error { return logger.file.Close() }

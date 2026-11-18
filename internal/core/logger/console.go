package logger

import (
	"fmt"

	"github.com/peggco/pegg/internal/core/config"
)

const DriverConsole = "console"

func init() {
	RegisterDriver(DriverConsole, func(cfg config.LoggerConfig) (Handler, error) {
		return NewConsoleHandler(cfg), nil
	})
}

type ConsoleHandler struct{}

func NewConsoleHandler(cfg config.LoggerConfig) *ConsoleHandler {
	return &ConsoleHandler{}
}

func (c *ConsoleHandler) Write(rec Record) error {
	fmt.Println(FormatRecord(rec))
	return nil
}

func (c *ConsoleHandler) Close() error {
	return nil
}

package logx

import (
	"fmt"
	"testing"
)

func TestLogxV2(t *testing.T) {
	log, err := NewLogger("logs/app.log", DEBUG, 1, true) // max 1MB
	if err != nil {
		panic(err)
	}
	defer log.Close()

	log.StartWorker()

	log.Info("This is an info message")
	log.Warn("This is a warning")
	log.Error("This is an error")

	for i := 0; i < 10; i++ {
		log.Debug(fmt.Sprintf("Log line %d", i))
	}
}

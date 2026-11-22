package logger

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestAllLoggersAreInitialised(t *testing.T) {
	loggers := map[string]interface{ Write([]byte) (int, error) }{
		"WS_S_LOGGER":      WS_S_LOGGER.Writer(),
		"WS_F_LOGGER":      WS_F_LOGGER.Writer(),
		"GRPC_S_LOGGER":    GRPC_S_LOGGER.Writer(),
		"GRPC_F_LOGGER":    GRPC_F_LOGGER.Writer(),
		"INTERNAL_LOGGER":  INTERNAL_LOGGER.Writer(),
		"SC_S_LOGGER":      SC_S_LOGGER.Writer(),
		"SC_F_LOGGER":      SC_F_LOGGER.Writer(),
		"SIGNATURE_LOGGER": SIGNATURE_LOGGER.Writer(),
	}

	for name, writer := range loggers {
		if writer == nil {
			t.Errorf("%s has no writer", name)
		}
	}
}

func TestSetOutputRedirectsEveryLogger(t *testing.T) {
	var buffer bytes.Buffer

	SetOutput(&buffer)
	defer SetOutput(io.Discard)

	WS_S_LOGGER.Println("ws success message")
	GRPC_F_LOGGER.Println("grpc failure message")
	INTERNAL_LOGGER.Println("internal message")

	output := buffer.String()
	for _, want := range []string{"ws success message", "grpc failure message", "internal message"} {
		if !strings.Contains(output, want) {
			t.Errorf("output %q does not contain %q", output, want)
		}
	}
}

func TestSetOutputToDiscardIsSafe(t *testing.T) {
	SetOutput(io.Discard)
	defer SetOutput(io.Discard)

	// Writing to a discarded logger must not panic.
	INTERNAL_LOGGER.Println("this goes nowhere")
	SC_F_LOGGER.Printf("formatted %d", 1)
}

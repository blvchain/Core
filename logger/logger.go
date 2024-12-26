package logger

import (
	"blvchain/core/config"
	"io"
	"log"
	"os"

	"github.com/natefinch/lumberjack"
)

// Paths
const (
	mainPath     string = "./log/"
	ws_success   string = mainPath + "websocket/success/log.log"
	ws_fail      string = mainPath + "websocket/fail"
	grpc_success string = mainPath + "gRPC/success"
	grpc_fail    string = mainPath + "gRPC/fail"
	internal     string = mainPath + "internal"
)

var (
	WS_S_LOGGER, WS_F_LOGGER, GRPC_S_LOGGER, GRPC_F_LOGGER, INTERNAL_LOGGER                                    *log.Logger
	ws_f_logger_output, ws_s_logger_output, grpc_s_logger_output, grpc_f_logger_output, internal_logger_output io.Writer
)

func init() {
	// Websocket success
	ws_s_logger := &lumberjack.Logger{
		Filename:  ws_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// Websocket fail
	ws_f_logger := &lumberjack.Logger{
		Filename:  ws_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	// gRPC success
	grpc_s_logger := &lumberjack.Logger{
		Filename:  grpc_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// gRPC fail
	grpc_f_logger := &lumberjack.Logger{
		Filename:  grpc_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	// Internal
	internal_logger := &lumberjack.Logger{
		Filename:  internal,
		MaxSize:   2,
		LocalTime: true,
	}

	// Dev logger
	if config.DEV_MODE {
		ws_s_logger_output = io.MultiWriter(ws_s_logger, os.Stdout)
		ws_f_logger_output = io.MultiWriter(ws_f_logger, os.Stderr)

		grpc_s_logger_output = io.MultiWriter(grpc_s_logger, os.Stdout)
		grpc_f_logger_output = io.MultiWriter(grpc_f_logger, os.Stderr)

		internal_logger_output = io.MultiWriter(internal_logger, os.Stderr)
	} else {
		ws_s_logger_output = ws_s_logger
		ws_f_logger_output = ws_f_logger

		grpc_s_logger_output = grpc_s_logger
		grpc_f_logger_output = grpc_f_logger

		internal_logger_output = internal_logger
	}

	// Logger
	WS_S_LOGGER = log.New(ws_s_logger_output, "", log.Ldate|log.Ltime)
	WS_F_LOGGER = log.New(ws_f_logger_output, "", log.Ldate|log.Ltime)

	GRPC_S_LOGGER = log.New(grpc_s_logger_output, "", log.Ldate|log.Ltime)
	GRPC_F_LOGGER = log.New(grpc_f_logger_output, "", log.Ldate|log.Ltime)

	INTERNAL_LOGGER = log.New(internal_logger_output, "", log.Ldate|log.Ltime)
}

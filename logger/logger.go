package logger

import "github.com/natefinch/lumberjack"

// Paths
const (
	mainPath     string = "./log/"
	ws_success   string = mainPath + "websocket"
	ws_fail      string = mainPath + "websocket"
	grpc_success string = mainPath + "gRPC"
	grpc_fail    string = mainPath + "gRPC"
	internal     string = mainPath + "internal"
)

var (
	// Websocket success
	WS_S_LOGGER = &lumberjack.Logger{
		Filename:  ws_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// Websocket fail
	WS_F_LOGGER = &lumberjack.Logger{
		Filename:  ws_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	// gRPC success
	GRPC_S_LOGGER = &lumberjack.Logger{
		Filename:  grpc_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// gRPC fail
	GRPC_F_LOGGER = &lumberjack.Logger{
		Filename:  grpc_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	// Internal
	INTERNAL_LOGGER = &lumberjack.Logger{
		Filename:  internal,
		MaxSize:   2,
		LocalTime: true,
	}
)

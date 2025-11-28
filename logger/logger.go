package logger

import (
	"io"
	"log"

	"github.com/natefinch/lumberjack"
)

// Paths
const (
	mainPath     string = "./log/"
	ws_success   string = mainPath + "websocket/success/log.log"
	ws_fail      string = mainPath + "websocket/fail/log.log"
	grpc_success string = mainPath + "gRPC/success/log.log"
	grpc_fail    string = mainPath + "gRPC/fail/log.log"
	sc_success   string = mainPath + "smartContract/success/log.log"
	sc_fail      string = mainPath + "smartContract/fail/log.log"
	wal_success  string = mainPath + "wal/success/log.log"
	wal_fail     string = mainPath + "wal/fail/log.log"
	vc_success   string = mainPath + "verifiableCredential/success/log.log"
	vc_fail      string = mainPath + "verifiableCredential/fail/log.log"
	internal     string = mainPath + "internal/log.log"
	signature    string = mainPath + "signature/log.log"
)

var (
	WS_S_LOGGER,
	WS_F_LOGGER,
	GRPC_S_LOGGER,
	GRPC_F_LOGGER,
	INTERNAL_LOGGER,
	SC_S_LOGGER,
	SC_F_LOGGER,
	VC_S_LOGGER,
	VC_F_LOGGER,
	WAL_S_LOGGER,
	WAL_F_LOGGER,
	SIGNATURE_LOGGER *log.Logger

	ws_f_logger_output,
	ws_s_logger_output,
	grpc_s_logger_output,
	grpc_f_logger_output,
	internal_logger_output,
	sc_s_logger_output,
	sc_f_logger_output,
	vc_s_logger_output,
	vc_f_logger_output,
	wal_s_logger_output,
	wal_f_logger_output,
	signature_logger_output io.Writer
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

	// Signature
	signature_logger := &lumberjack.Logger{
		Filename:  signature,
		MaxSize:   2,
		LocalTime: true,
	}

	// Smart contract success
	sc_s_logger := &lumberjack.Logger{
		Filename:  sc_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// Smart contract fail
	sc_f_logger := &lumberjack.Logger{
		Filename:  sc_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	// Verifiable credential success
	vc_s_logger := &lumberjack.Logger{
		Filename:  vc_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// Verifiable credential fail
	vc_f_logger := &lumberjack.Logger{
		Filename:  vc_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	// Wal success
	wal_s_logger := &lumberjack.Logger{
		Filename:  vc_success,
		MaxSize:   2,
		LocalTime: true,
	}
	// Wal fail
	wal_f_logger := &lumberjack.Logger{
		Filename:  vc_fail,
		MaxSize:   2,
		LocalTime: true,
	}

	ws_s_logger_output = ws_s_logger
	ws_f_logger_output = ws_f_logger

	grpc_s_logger_output = grpc_s_logger
	grpc_f_logger_output = grpc_f_logger

	internal_logger_output = internal_logger
	signature_logger_output = signature_logger

	sc_s_logger_output = sc_s_logger
	sc_f_logger_output = sc_f_logger

	vc_s_logger_output = vc_s_logger
	vc_f_logger_output = vc_f_logger

	wal_s_logger_output = wal_s_logger
	wal_f_logger_output = wal_f_logger

	// Logger
	WS_S_LOGGER = log.New(ws_s_logger_output, "", log.Ldate|log.Ltime)
	WS_F_LOGGER = log.New(ws_f_logger_output, "", log.Ldate|log.Ltime)

	GRPC_S_LOGGER = log.New(grpc_s_logger_output, "", log.Ldate|log.Ltime)
	GRPC_F_LOGGER = log.New(grpc_f_logger_output, "", log.Ldate|log.Ltime)

	INTERNAL_LOGGER = log.New(internal_logger_output, "", log.Ldate|log.Ltime)
	SIGNATURE_LOGGER = log.New(signature_logger_output, "", log.Ldate|log.Ltime)

	SC_S_LOGGER = log.New(sc_s_logger_output, "", log.Ldate|log.Ltime)
	SC_F_LOGGER = log.New(sc_f_logger_output, "", log.Ldate|log.Ltime)

	VC_S_LOGGER = log.New(vc_s_logger_output, "", log.Ldate|log.Ltime)
	VC_F_LOGGER = log.New(vc_f_logger_output, "", log.Ldate|log.Ltime)

	WAL_S_LOGGER = log.New(wal_s_logger_output, "", log.Ldate|log.Ltime)
	WAL_F_LOGGER = log.New(wal_f_logger_output, "", log.Ldate|log.Ltime)
}

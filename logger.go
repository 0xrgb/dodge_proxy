package main

import (
	"io"
	"log"
)

// Logger
var (
	Trace *log.Logger // Lv0: 현재 동작 자세히 설명
	Info  *log.Logger // Lv1: 무언가 변화가 생김
	Error *log.Logger // Lv2: 알아야 할 문제가 생김
	// Lv3: panic
)

func initLogger(traceWriter io.Writer, infoWriter io.Writer, errorWriter io.Writer, flag int) {
	// Logger들 초기화
	Trace = log.New(traceWriter, "[TRACE] ", flag)
	Info = log.New(infoWriter, "[INFO] ", flag)
	Error = log.New(errorWriter, "[ERROR] ", flag)
}

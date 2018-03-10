package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
)

func usage() {
	fmt.Println("Usage: dodge_proxy [-vq] <PORT>")
	os.Exit(0)
}

func parseArgs() int {
	if n := len(os.Args); !(2 <= n && n <= 3) {
		usage()
	}

	logLv := 1
	port := -1
	for _, val := range os.Args[1:] {
		switch val {
		case "-v":
			logLv = 0
		case "-q":
			logLv = 2
		default:
			xport, err := strconv.Atoi(val)
			if err != nil {
				usage()
			}
			port = xport
		}
	}
	if port < 0 {
		usage()
	}

	switch logLv {
	case 0:
		initLogger(os.Stdout, os.Stdout, os.Stderr, log.Ltime)
	case 1:
		initLogger(ioutil.Discard, os.Stdout, os.Stderr, log.Ltime)
	case 2:
		initLogger(ioutil.Discard, ioutil.Discard, os.Stderr, log.Ltime)
	}

	return port
}

func main() {
	port := parseArgs()

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		Error.Fatalln("Cannot open tcp server")
	}
	Info.Println("Proxy is now on port", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			Error.Println("Cannot resolve connection:", err)
		} else {
			go dodgeHTTP(conn)
		}
	}
}

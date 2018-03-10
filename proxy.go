package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

// dummyPacket 은 HTTP Blocking 우회를 위해 사용되는 가짜 HTTP 패킷이다.
const dummyPacket = "GET / HTTP/1.1\r\n" +
	"Host: dummy.host\r\n" +
	"Connection: keep-alive\r\n\r\n"

// TODO: 상수 설명하기
const (
	pfContentLength  = "Content-Length: "
	pfHosts          = "Host: "
	maxContentLength = 1 << 26 // 64MiB
)

var (
	pfbContentLength = []byte(pfContentLength)
	pfbHosts         = []byte(pfHosts)
)

// readHTTPResponse 는 bufio.Reader에서 서버로부터 들어오는 HTTP Response를 하나 읽는다.
// 그 후, 읽은 패킷이 담긴 []byte를 반환한다.
func readHTTPResponse(reader *bufio.Reader) ([]byte, error) {
	packet := make([]byte, 0, 1600)
	contentLength := -1

	for {
		// TODO: 연결이 중간에 끊겼을 때, 제대로 처리하기
		// TODO: 잘못된 TCP Response로 인하여 너무 많이 읽는 걸 방지하기
		headerLine, err := reader.ReadBytes('\n')
		if err != nil {
			Error.Println("Connection ended in HTTP Response:", err)
			return nil, err
		}

		packet = append(packet, headerLine...)
		if len(headerLine) == 2 {
			break
		}

		if bytes.HasPrefix(headerLine, pfbContentLength) {
			if contentLength >= 0 {
				return nil, errors.New("Too many Content-Length")
			}
			contentLength, err = strconv.Atoi(
				string(headerLine[len(pfbContentLength) : len(headerLine)-2]))
			if err != nil {
				return nil, errors.New("Bad Content-Length")
			}
		}
	}

	// 헤더에 Content-Length가 없다면, len이 0이라는 뜻이므로, 더 이상 읽을 필요가 없다.
	if contentLength < 0 {
		return packet, nil
	}

	if contentLength > maxContentLength {
		return nil, fmt.Errorf("Content-Length: %d is too long", contentLength)
	}

	// contentLength 개의 바이트를 추가로 읽는다.
	tmp := make([]byte, contentLength)
	_, err := io.ReadFull(reader, tmp)
	if err != nil {
		return nil, errors.New("HTTP Response is too short")
	}

	packet = append(packet, tmp...)
	return packet, nil
}

// readHTTPRequest 는 bufio.Reader에서 클라이언트가 요청하는 HTTP Request를 하나 읽는다.
// 그 후, 읽은 패킷이 담긴 []byte를 Return 한다.
func readHTTPRequest(reader *bufio.Reader) ([]byte, []byte) {
	// TODO: 문제가 되는 입력에 대해서 에러 처리
	var host []byte
	contentLength := -1
	packet := make([]byte, 0, 560)

	for {
		headerLine, err := reader.ReadBytes('\n')
		if err != nil {
			// 헤더를 읽지 못한 상태로 통신이 끝남.
			return nil, nil
		}

		// 반환할 패킷에 읽은 줄을 추가한다.
		packet = append(packet, headerLine...)
		if len(headerLine) == 2 {
			break
		}

		// Host
		if bytes.HasPrefix(headerLine, pfbHosts) {
			if host != nil {
				return nil, nil
			}
			host = headerLine[len(pfbHosts) : len(headerLine)-2]
		} else if bytes.HasPrefix(headerLine, pfbContentLength) {
			if contentLength >= 0 {
				return nil, nil
			}
			contentLength, err = strconv.Atoi(
				string(headerLine[len(pfbContentLength) : len(headerLine)-2]))
			if err != nil {
				return nil, nil
			}
		}
	}

	if host == nil {
		return nil, nil
	}

	if contentLength < 0 {
		return packet, host
	}

	if contentLength > maxContentLength {
		return nil, nil
	}

	// contentLength 개의 바이트를 추가로 읽는다.
	tmp := make([]byte, contentLength)
	_, err := io.ReadFull(reader, tmp)
	if err != nil {
		return nil, nil
	}

	packet = append(packet, tmp...)
	return packet, host
}

// dodgeHTTP 는 HTTP Connection을 받아 HTTP 통신을 주도한다.
func dodgeHTTP(conn net.Conn) {
	connID := getUID()
	Trace.Printf("Connection started (id: %d)\n", connID)

	defer func(x uint) {
		Trace.Printf("Connection closed (id: %d)\n", x)
		conn.Close()
	}(connID)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	buf := make([]byte, 0, 1600)
	buf = append(buf, []byte(dummyPacket)...)

	for {
		packet, host := readHTTPRequest(reader)
		if packet == nil {
			break
		}
		shost := string(host)

		// HTTP Request 파싱에 성공했다.
		Trace.Println("HOST:", shost, "/ len:", len(packet))

		// 해당 호스트와 통신하기 위해 소켓을 연다.
		// HOST에 포트 번호가 적혀있지 않다면, HTTP 포트를 이용한다.
		// 현재는 HOST 뒤에 포트 번호가 없다고 가정하고 진행한다.
		// TODO: host에서 포트번호 확인
		dial, err := net.Dial("tcp", shost+":http")
		if err != nil {
			Error.Println("Cannot open socket to ", shost)
			continue
		}

		// Dummy + 유저 HTTP Request를 서버에 보낸다.
		nbuf := append(buf[:len(dummyPacket)], packet...)
		dialWriter := bufio.NewWriter(dial)
		_, err = dialWriter.Write(nbuf)
		if err != nil {
			Error.Println("Send failed:", err)
			dial.Close()
			continue
		}
		dialWriter.Flush()
		Trace.Println("Send Dummy HTTP Request to", shost)

		// 서버의 답장에서 Dummy 부분을 지운다.
		dialReader := bufio.NewReader(dial)
		_, err = readHTTPResponse(dialReader)
		if err != nil {
			Error.Println("Fail to parse dummy HTTP Response:", err)
			dial.Close()
			continue
		}
		Trace.Println("Recieve Dummy HTTP Response from", shost)

		resp, err := readHTTPResponse(dialReader)
		if err != nil {
			Error.Println("Fail to parse HTTP Response:", err)
		}
		Trace.Println("Recieve HTTP Response from", shost)

		// 유저에게 받은 패킷을 돌려준다
		_, err = writer.Write(resp)
		if err != nil {
			Error.Println("Fail to relay:", err)
		}
		writer.Flush()
		Trace.Println("Relay HTTP Response to from", shost)
	}
}

package server

import (
	"bytes"
	"golang.org/x/net/dns/dnsmessage"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

type OverseaDnsServer struct {
	listener net.Listener
}

func NewOverseaDnsServer() *OverseaDnsServer {
	listener, err := net.Listen("unix", "/tmp/oversea_dns.sock")

	if err != nil {
		log.Panicln(err)
	}

	return &OverseaDnsServer{listener: listener}
}

func (s *OverseaDnsServer) Start() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *OverseaDnsServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		message := make([]byte, 1024)
		_, err := conn.Read(message)

		if err != nil && err == io.EOF {
			log.Println(err)
			return
		}

		var m dnsmessage.Message

		err = m.Unpack(message)

		if err != nil {
			log.Println(err)
			continue
		}

		go s.handleMessage(m, conn)
	}
}

func (s *OverseaDnsServer) handleMessage(m dnsmessage.Message, conn net.Conn) {
	resp, err := HandleOverseaDNSQuery(m)
	_, err = conn.Write(resp)

	if err != nil {
		log.Println(err)
		return
	}
}

func HandleOverseaDNSQuery(m dnsmessage.Message) ([]byte, error) {
	client := &http.Client{}

	dnsMessage, err := m.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://8.8.8.8/dns-query", bytes.NewBuffer(dnsMessage))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err

	}
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return body, nil
}

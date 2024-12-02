package client

import (
	"golang.org/x/net/dns/dnsmessage"
	"net"
)

type DNSClient struct {
	serverAddr string
}

func NewDNSClient(serverAddr string) *DNSClient {
	return &DNSClient{
		serverAddr: serverAddr,
	}
}

func (c *DNSClient) Request(m dnsmessage.Message) ([]byte, error) {
	conn, err := net.Dial("udp", c.serverAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dnsMessage, err := m.Pack()
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(dnsMessage)
	if err != nil {
		return nil, err
	}

	response := make([]byte, 512)
	n, err := conn.Read(response)
	if err != nil {
		return nil, err
	}

	return response[:n], nil
} 
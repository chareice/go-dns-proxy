package client

import (
	"context"
	"net"

	"golang.org/x/net/dns/dnsmessage"
)

type UDPClient struct {
	serverAddr string
}

func NewUDPClient(serverAddr string) *UDPClient {
	return &UDPClient{serverAddr: serverAddr}
}

func (c *UDPClient) Request(ctx context.Context, m dnsmessage.Message) ([]byte, error) {
	packed, err := m.Pack()
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("udp", c.serverAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_, err = conn.Write(packed)
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

func (c *UDPClient) String() string {
	return c.serverAddr
} 
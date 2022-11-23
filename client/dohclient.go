package client

import (
	"bytes"
	"golang.org/x/net/dns/dnsmessage"
	"io/ioutil"
	"net/http"
)

type DOHClient struct {
	serverAddr string
}

func NewDOHClient(serverAddr string) *DOHClient {
	return &DOHClient{
		serverAddr: serverAddr,
	}
}

func (c *DOHClient) Request(m dnsmessage.Message) ([]byte, error) {
	client := &http.Client{}

	dnsMessage, err := m.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.serverAddr, bytes.NewBuffer(dnsMessage))
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

package client

import "golang.org/x/net/dns/dnsmessage"

type DNSResolver interface {
	Request(m dnsmessage.Message) ([]byte, error)
} 
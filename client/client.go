package client

import (
	"context"

	"golang.org/x/net/dns/dnsmessage"
)

type DNSResolver interface {
	Request(ctx context.Context, m dnsmessage.Message) ([]byte, error)
} 
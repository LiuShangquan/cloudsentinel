package probe

import (
	"context"
	"errors"
	"net"
)

var ErrNetworkDenied = errors.New("network target denied")

type NetworkPolicy struct {
	AllowPrivate  bool
	AllowLoopback bool
	Resolver      *net.Resolver
}

func (p NetworkPolicy) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	if literal := net.ParseIP(host); literal != nil {
		if err := p.validate(literal); err != nil {
			return nil, err
		}
		return []net.IP{literal}, nil
	}
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, &net.DNSError{Name: host, Err: "no addresses"}
	}
	for _, address := range addresses {
		if err := p.validate(address); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func (p NetworkPolicy) validate(address net.IP) error {
	metadata := net.ParseIP("169.254.169.254")
	if address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.Equal(metadata) {
		return ErrNetworkDenied
	}
	if address.IsLoopback() && !p.AllowLoopback {
		return ErrNetworkDenied
	}
	if address.IsPrivate() && !p.AllowPrivate {
		return ErrNetworkDenied
	}
	return nil
}

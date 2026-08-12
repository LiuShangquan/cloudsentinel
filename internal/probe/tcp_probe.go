package probe

import (
	"context"
	"errors"
	"net"
)

type TCPProbe struct {
	policy NetworkPolicy
	dialer net.Dialer
}

func NewTCPProbe(policy NetworkPolicy) *TCPProbe { return &TCPProbe{policy: policy} }

func (p *TCPProbe) Execute(ctx context.Context, target Target) (Result, error) {
	host, port, err := net.SplitHostPort(target.Address)
	if err != nil || host == "" || port == "" {
		err = errors.New("invalid TCP target")
		return Result{ErrorCode: "network_error", ErrorMessage: err.Error()}, err
	}
	addresses, err := p.policy.Resolve(ctx, host)
	if err != nil {
		return classified(err)
	}
	var last error
	for _, address := range addresses {
		connection, dialErr := p.dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), port))
		if dialErr == nil {
			if closeErr := connection.Close(); closeErr != nil {
				return classified(closeErr)
			}
			return Result{Success: true}, nil
		}
		last = dialErr
	}
	return classified(last)
}

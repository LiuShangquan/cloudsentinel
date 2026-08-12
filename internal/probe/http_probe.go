package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
)

type HTTPProbe struct {
	client *http.Client
	policy NetworkPolicy
}

func NewHTTPProbe(policy NetworkPolicy) *HTTPProbe {
	dialer := &net.Dialer{}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := policy.Resolve(ctx, host)
			if err != nil {
				return nil, err
			}
			var last error
			for _, resolved := range addresses {
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				last = dialErr
			}
			return nil, last
		},
		ForceAttemptHTTP2: true,
	}
	probe := &HTTPProbe{policy: policy}
	probe.client = &http.Client{Transport: transport, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit exceeded")
		}
		_, err := policy.Resolve(request.Context(), request.URL.Hostname())
		return err
	}}
	return probe
}

func (p *HTTPProbe) Execute(ctx context.Context, target Target) (Result, error) {
	parsed, err := url.Parse(target.Address)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return Result{ErrorCode: "network_error", ErrorMessage: "invalid HTTP target"}, errors.New("invalid HTTP target")
	}
	if _, err := p.policy.Resolve(ctx, parsed.Hostname()); err != nil {
		return classified(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Result{ErrorCode: "network_error", ErrorMessage: "create request"}, err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return classified(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 399 {
		return Result{Success: false, HTTPStatusCode: response.StatusCode, ErrorCode: "http_status_error", ErrorMessage: fmt.Sprintf("HTTP status %d", response.StatusCode)}, nil
	}
	return Result{Success: true, HTTPStatusCode: response.StatusCode}, nil
}

func classified(err error) (Result, error) {
	code := "network_error"
	var dnsError *net.DNSError
	var netError net.Error
	var tlsError tls.RecordHeaderError
	switch {
	case errors.Is(err, ErrNetworkDenied):
		code = "network_denied"
	case errors.As(err, &dnsError):
		code = "dns_error"
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.As(err, &netError) && netError.Timeout():
		code = "timeout"
	case errors.As(err, &tlsError) || strings.Contains(strings.ToLower(err.Error()), "tls") || strings.Contains(strings.ToLower(err.Error()), "certificate"):
		code = "tls_error"
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "connection refused") || strings.Contains(strings.ToLower(err.Error()), "actively refused"):
		code = "connection_refused"
	}
	return Result{Success: false, ErrorCode: code, ErrorMessage: err.Error()}, err
}

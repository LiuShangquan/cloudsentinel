package asset

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var ErrInvalidInput = errors.New("invalid input")

func validateHostAddress(address string) error {
	if address == "" || strings.ContainsAny(address, "/@") || strings.Contains(address, "://") {
		return ErrInvalidInput
	}
	if net.ParseIP(address) != nil {
		return nil
	}
	if strings.Contains(address, ":") || !validHostname(address) {
		return ErrInvalidInput
	}
	return nil
}

func validHostname(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func validateTarget(kind, target string) error {
	switch kind {
	case TypeHTTP:
		parsed, err := url.Parse(target)
		if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
			return ErrInvalidInput
		}
		if port := parsed.Port(); port != "" {
			value, err := strconv.Atoi(port)
			if err != nil || value < 1 || value > 65535 {
				return ErrInvalidInput
			}
		}
		return nil
	case TypeTCP:
		host, port, err := net.SplitHostPort(target)
		if err != nil || host == "" {
			return ErrInvalidInput
		}
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return ErrInvalidInput
		}
		if net.ParseIP(host) == nil && !validHostname(host) {
			return ErrInvalidInput
		}
		return nil
	default:
		return ErrInvalidInput
	}
}

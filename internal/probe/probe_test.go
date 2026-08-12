package probe

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func loopbackPolicy() NetworkPolicy { return NetworkPolicy{AllowPrivate: true, AllowLoopback: true} }

func TestHTTPProbeStatusesRedirectAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ok":
			writer.WriteHeader(http.StatusNoContent)
		case "/redirect":
			http.Redirect(writer, request, "/ok", http.StatusMovedPermanently)
		case "/slow":
			<-request.Context().Done()
		case "/error":
			writer.WriteHeader(http.StatusInternalServerError)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	implementation := NewHTTPProbe(loopbackPolicy())

	for _, test := range []struct {
		path    string
		success bool
		status  int
	}{{"/ok", true, 204}, {"/redirect", true, 204}, {"/missing", false, 404}, {"/error", false, 500}} {
		result, _ := implementation.Execute(context.Background(), Target{Type: "http", Address: server.URL + test.path})
		if result.Success != test.success || result.HTTPStatusCode != test.status {
			t.Errorf("%s result=%+v", test.path, result)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	result, _ := implementation.Execute(ctx, Target{Type: "http", Address: server.URL + "/slow"})
	if result.ErrorCode != "timeout" {
		t.Fatalf("timeout result=%+v", result)
	}
}

func TestHTTPProbeTLSErrorAndRedirectLimit(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tlsServer.Close()
	implementation := NewHTTPProbe(loopbackPolicy())
	result, _ := implementation.Execute(context.Background(), Target{Type: "http", Address: tlsServer.URL})
	if result.ErrorCode != "tls_error" {
		t.Fatalf("TLS result=%+v", result)
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, request.URL.Path, http.StatusFound)
	}))
	defer redirect.Close()
	result, err := implementation.Execute(context.Background(), Target{Type: "http", Address: redirect.URL + "/loop"})
	if err == nil || result.Success {
		t.Fatalf("redirect limit result=%+v err=%v", result, err)
	}
}

func TestNetworkPolicyDeniesMetadataAndLoopback(t *testing.T) {
	policy := NetworkPolicy{AllowPrivate: true, AllowLoopback: false}
	for _, host := range []string{"169.254.169.254", "127.0.0.1", "0.0.0.0", "224.0.0.1"} {
		if _, err := policy.Resolve(context.Background(), host); !errors.Is(err, ErrNetworkDenied) {
			t.Errorf("Resolve(%s) err=%v", host, err)
		}
	}
}

func TestTCPProbeOpenClosedAndCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	implementation := NewTCPProbe(loopbackPolicy())
	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			connection.Close()
		}
		close(accepted)
	}()
	result, err := implementation.Execute(context.Background(), Target{Type: "tcp", Address: address})
	if err != nil || !result.Success {
		t.Fatalf("open TCP result=%+v err=%v", result, err)
	}
	<-accepted
	listener.Close()
	result, _ = implementation.Execute(context.Background(), Target{Type: "tcp", Address: address})
	if result.Success || result.ErrorCode != "connection_refused" {
		t.Fatalf("closed TCP result=%+v", result)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	result, _ = implementation.Execute(canceled, Target{Type: "tcp", Address: address})
	if result.Success {
		t.Fatal("canceled TCP probe succeeded")
	}
}

type sequenceProbe struct {
	results []Result
	errors  []error
	calls   int
}

func (p *sequenceProbe) Execute(context.Context, Target) (Result, error) {
	index := p.calls
	p.calls++
	return p.results[index], p.errors[index]
}

func TestRetryPolicy(t *testing.T) {
	fiveHundred := &sequenceProbe{results: []Result{{HTTPStatusCode: 500}, {Success: true, HTTPStatusCode: 200}}, errors: []error{nil, nil}}
	result, attempts := ExecuteWithRetry(context.Background(), fiveHundred, Target{}, time.Second, 2, time.Millisecond)
	if !result.Success || attempts != 2 {
		t.Fatalf("5xx retry result=%+v attempts=%d", result, attempts)
	}
	fourHundred := &sequenceProbe{results: []Result{{HTTPStatusCode: 404}}, errors: []error{nil}}
	_, attempts = ExecuteWithRetry(context.Background(), fourHundred, Target{}, time.Second, 3, time.Millisecond)
	if attempts != 1 {
		t.Fatalf("4xx attempts=%d", attempts)
	}
	failed := &sequenceProbe{results: []Result{{ErrorCode: "timeout"}, {ErrorCode: "timeout"}, {ErrorCode: "timeout"}}, errors: []error{context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded}}
	_, attempts = ExecuteWithRetry(context.Background(), failed, Target{}, time.Second, 2, time.Millisecond)
	if attempts != 3 {
		t.Fatalf("max retry attempts=%d", attempts)
	}
}

func TestBoundedEnqueueHonorsCancellation(t *testing.T) {
	jobs := make(chan streamJob, 1)
	if !enqueue(context.Background(), jobs, streamJob{}) {
		t.Fatal("first enqueue failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if enqueue(ctx, jobs, streamJob{}) {
		t.Fatal("enqueue bypassed bounded backpressure")
	}
}

package protos

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestRateLimiterBurstThenReject(t *testing.T) {
	rl := NewRateLimiter(
		map[string]MethodLimit{"/test/Method": {R: rate.Limit(1), Burst: 2}},
		MethodLimit{R: rate.Limit(1), Burst: 1},
		time.Minute,
	)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))

	for i := 0; i < 2; i++ {
		if !rl.getLimiter("/test/Method", clientKey(ctx)).Allow() {
			t.Fatalf("request %d should have been allowed by the burst", i+1)
		}
	}
	if rl.getLimiter("/test/Method", clientKey(ctx)).Allow() {
		t.Fatal("the third request should have been rejected")
	}
}

func TestRateLimiterIsolatesClients(t *testing.T) {
	rl := NewRateLimiter(
		map[string]MethodLimit{"/test/Method": {R: rate.Limit(0), Burst: 1}},
		MethodLimit{R: rate.Limit(0), Burst: 1},
		time.Minute,
	)

	a := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))
	b := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-b"))

	if !rl.getLimiter("/test/Method", clientKey(a)).Allow() {
		t.Fatal("client a should get its own burst")
	}
	if !rl.getLimiter("/test/Method", clientKey(b)).Allow() {
		t.Fatal("client b should get its own burst, independent of client a")
	}
	if rl.getLimiter("/test/Method", clientKey(a)).Allow() {
		t.Fatal("client a exhausted its burst")
	}
}

func TestRateLimiterUsesPerMethodConfig(t *testing.T) {
	rl := NewRateLimiter(
		map[string]MethodLimit{"/test/Bursty": {R: rate.Limit(0), Burst: 5}},
		MethodLimit{R: rate.Limit(0), Burst: 1},
		time.Minute,
	)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))
	key := clientKey(ctx)

	for i := 0; i < 5; i++ {
		if !rl.getLimiter("/test/Bursty", key).Allow() {
			t.Fatalf("bursty method should allow request %d", i+1)
		}
	}
	if !rl.getLimiter("/test/Other", key).Allow() {
		t.Fatal("the default config should still apply to unlisted methods")
	}
	if rl.getLimiter("/test/Other", key).Allow() {
		t.Fatal("the default burst of 1 is exhausted")
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(nil, MethodLimit{R: rate.Limit(1), Burst: 1}, 40*time.Millisecond)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))
	_ = rl.getLimiter("/test/Method", clientKey(ctx))

	rl.mu.Lock()
	initial := len(rl.limiters)
	rl.mu.Unlock()
	if initial != 1 {
		t.Fatalf("limiter map has %d entries, want 1", initial)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rl.mu.Lock()
		size := len(rl.limiters)
		rl.mu.Unlock()
		if size == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	rl.mu.Lock()
	size := len(rl.limiters)
	rl.mu.Unlock()
	t.Fatalf("idle entries were not cleaned up: %d left", size)
}

func TestUnaryServerInterceptor(t *testing.T) {
	rl := NewRateLimiter(
		map[string]MethodLimit{"/test/Method": {R: rate.Limit(0), Burst: 1}},
		MethodLimit{R: rate.Limit(0), Burst: 1},
		time.Minute,
	)
	interceptor := rl.UnaryServerInterceptor()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}
	handler := func(context.Context, any) (any, error) { return "ok", nil }

	if _, err := interceptor(ctx, nil, info, handler); err != nil {
		t.Fatalf("first call was rejected: %v", err)
	}

	if _, err := interceptor(ctx, nil, info, handler); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second call = %v, want ResourceExhausted", err)
	}
}

type fakeStream struct {
	ctx context.Context
	grpc.ServerStream
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func TestStreamServerInterceptor(t *testing.T) {
	rl := NewRateLimiter(
		map[string]MethodLimit{"/test/Stream": {R: rate.Limit(0), Burst: 1}},
		MethodLimit{R: rate.Limit(0), Burst: 1},
		time.Minute,
	)
	interceptor := rl.StreamServerInterceptor()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))
	stream := &fakeStream{ctx: ctx}
	info := &grpc.StreamServerInfo{FullMethod: "/test/Stream"}
	handler := func(any, grpc.ServerStream) error { return nil }

	if err := interceptor(nil, stream, info, handler); err != nil {
		t.Fatalf("first stream was rejected: %v", err)
	}
	if err := interceptor(nil, stream, info, handler); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second stream = %v, want ResourceExhausted", err)
	}
}

func TestClientKey(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "secret"))
	if got := clientKey(ctx); got != "authorization:secret" {
		t.Errorf("clientKey = %q", got)
	}

	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-api-key", "secret"))
	if got := clientKey(ctx); got != "x-api-key:secret" {
		t.Errorf("clientKey = %q", got)
	}

	// Without metadata the peer address is used.
	peerCtx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 5555},
	})
	if got := clientKey(peerCtx); got != "ip:192.0.2.10" {
		t.Errorf("clientKey = %q", got)
	}

	if got := clientKey(context.Background()); got != "unknown" {
		t.Errorf("clientKey without metadata or peer = %q", got)
	}
}

func TestNewRateLimiterWithZeroTTLDoesNotSpin(t *testing.T) {
	// A non-positive ttl disables the cleanup goroutine; constructing one must
	// not panic.
	rl := NewRateLimiter(nil, MethodLimit{R: rate.Limit(1), Burst: 1}, 0)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "client-a"))
	if !rl.getLimiter("/test/Method", clientKey(ctx)).Allow() {
		t.Fatal("first request should be allowed")
	}
}

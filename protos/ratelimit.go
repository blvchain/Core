package protos

import (
	"context"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// MethodLimit describes rate and burst for a method.
type MethodLimit struct {
	R     rate.Limit // tokens per second
	Burst int
}

// RateLimiter holds per-method/per-client limiter entries.
type RateLimiter struct {
	mu           sync.Mutex
	limiters     map[string]*limiterEntry // key: method + "|" + clientKey
	methodConfig map[string]MethodLimit   // method -> limit configuration
	defaultCfg   MethodLimit
	ttl          time.Duration
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter takes a per-method config (method full names like "/gate.AddData/addData")
// and a default config used for methods not present in the map. ttl is used to cleanup idle entries.
func NewRateLimiter(methodConfig map[string]MethodLimit, defaultCfg MethodLimit, ttl time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters:     make(map[string]*limiterEntry),
		methodConfig: methodConfig,
		defaultCfg:   defaultCfg,
		ttl:          ttl,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) getConfig(method string) MethodLimit {
	if c, ok := rl.methodConfig[method]; ok {
		return c
	}
	return rl.defaultCfg
}

// clientKey extracts client identity: tries authorization header, x-api-key, then peer IP.
// You can modify to parse JWT to extract user id for per-user limits.
func clientKey(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("authorization"); len(vals) > 0 {
			return "auth:" + vals[0]
		}
		if vals := md.Get("x-api-key"); len(vals) > 0 {
			return "apikey:" + vals[0]
		}
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		if addr, ok := p.Addr.(*net.TCPAddr); ok {
			return "ip:" + addr.IP.String()
		}
		return "peer:" + p.Addr.String()
	}
	return "unknown"
}

func (rl *RateLimiter) getLimiter(method string, cKey string) *rate.Limiter {
	composite := method + "|" + cKey
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if e, ok := rl.limiters[composite]; ok {
		e.lastSeen = time.Now()
		return e.limiter
	}
	cfg := rl.getConfig(method)
	l := rate.NewLimiter(cfg.R, cfg.Burst)
	rl.limiters[composite] = &limiterEntry{limiter: l, lastSeen: time.Now()}
	return l
}

func (rl *RateLimiter) cleanupLoop() {
	if rl.ttl <= 0 {
		return
	}
	ticker := time.NewTicker(rl.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-rl.ttl)
		rl.mu.Lock()
		for k, e := range rl.limiters {
			if e.lastSeen.Before(cutoff) {
				delete(rl.limiters, k)
			}
		}
		rl.mu.Unlock()
	}
}

// UnaryServerInterceptor enforces limits per (method, client)
func (rl *RateLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		cKey := clientKey(ctx)
		l := rl.getLimiter(info.FullMethod, cKey)
		if !l.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor enforces limits for streaming RPCs the same way.
func (rl *RateLimiter) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		cKey := clientKey(ss.Context())
		l := rl.getLimiter(info.FullMethod, cKey)
		if !l.Allow() {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(srv, ss)
	}
}

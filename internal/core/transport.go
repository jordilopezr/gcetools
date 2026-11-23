package core

import (
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

// retryTransport rate-limits every request and retries idempotent GETs on
// 429 / 5xx / network errors with exponential backoff and jitter. All calls
// made by this tool are GETs, so retrying them is always safe.
//
// Each API host has its own limiter, so a slow module (Monitoring,
// Recommender) does not hold back the Compute listings and the other way
// round.
type retryTransport struct {
	base       http.RoundTripper
	limiters   *hostLimiters
	maxRetries int
}

// computeFactor multiplies the rate for compute.googleapis.com: its read
// quota is charged to each scanned project, and the scan reads several
// projects at a time.
const computeFactor = 3

type hostLimiters struct {
	rps float64
	mu  sync.Mutex
	m   map[string]*rate.Limiter
}

func newHostLimiters(rps float64) *hostLimiters {
	return &hostLimiters{rps: rps, m: map[string]*rate.Limiter{}}
}

func (h *hostLimiters) get(host string) *rate.Limiter {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := h.m[host]
	if l == nil {
		r := h.rps
		if host == "compute.googleapis.com" {
			r *= computeFactor
		}
		l = rate.NewLimiter(rate.Limit(r), int(r)+1)
		h.m[host] = l
	}
	return l
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for attempt := 0; ; attempt++ {
		if err := t.limiters.get(req.URL.Host).Wait(ctx); err != nil {
			return nil, err
		}
		resp, err := t.base.RoundTrip(req)
		retryable := req.Method == http.MethodGet && attempt < t.maxRetries && ctx.Err() == nil &&
			(err != nil || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500)
		if !retryable {
			return resp, err
		}
		wait := backoff(attempt)
		if resp != nil {
			if s, convErr := strconv.Atoi(resp.Header.Get("Retry-After")); convErr == nil && s > 0 && s < 120 {
				wait = time.Duration(s) * time.Second
			}
			io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func backoff(attempt int) time.Duration {
	d := time.Duration(500<<attempt) * time.Millisecond
	if d > 20*time.Second {
		d = 20 * time.Second
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

// NewHTTPClient returns an authenticated client with rate limiting (rps per
// API, three times that for Compute) and retries.
func NewHTTPClient(ts oauth2.TokenSource, rps float64) *http.Client {
	if rps <= 0 {
		rps = 20
	}
	return &http.Client{
		Timeout: 2 * time.Minute,
		Transport: &oauth2.Transport{
			Source: ts,
			Base: &retryTransport{
				base:       http.DefaultTransport,
				limiters:   newHostLimiters(rps),
				maxRetries: 6,
			},
		},
	}
}

// quotaTransport names the project that pays for quota (x-goog-user-project).
type quotaTransport struct {
	base    http.RoundTripper
	project string
}

func (t *quotaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("X-Goog-User-Project", t.project)
	return t.base.RoundTrip(r)
}

// withQuotaProject returns a client that shares c's transport (and therefore
// its rate limiter) but adds the quota project header.
func withQuotaProject(c *http.Client, project string) *http.Client {
	if project == "" {
		return c
	}
	cp := *c
	cp.Transport = &quotaTransport{base: c.Transport, project: project}
	return &cp
}

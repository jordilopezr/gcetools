package core

import (
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

// retryTransport rate-limits every request and retries idempotent GETs on
// 429 / 5xx / network errors with exponential backoff and jitter. All calls
// made by this tool are GETs, so retrying them is always safe.
type retryTransport struct {
	base       http.RoundTripper
	limiter    *rate.Limiter
	maxRetries int
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for attempt := 0; ; attempt++ {
		if err := t.limiter.Wait(ctx); err != nil {
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

// NewHTTPClient returns an authenticated client with rate limiting and retries.
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
				limiter:    rate.NewLimiter(rate.Limit(rps), int(rps)+1),
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

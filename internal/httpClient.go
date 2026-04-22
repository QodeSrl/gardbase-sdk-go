package internal

import (
	"net/http"
	"time"
)

type TenantRoundTripper struct {
	Base                http.RoundTripper
	TenantID            string
	APIKey              string
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

func (t TenantRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Tenant-ID", t.TenantID)
	req.Header.Set("X-API-Key", t.APIKey)
	return t.base().RoundTrip(req)
}

func (t TenantRoundTripper) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func NewHttpClient(tenantID string, apiKey string, httpTimeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: httpTimeout,
		Transport: TenantRoundTripper{
			TenantID:            tenantID,
			APIKey:              apiKey,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

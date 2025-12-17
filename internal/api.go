package internal

import (
	"net/http"
	"time"
)

type APIClient struct {
	APIEndpoint string

	httpClient *http.Client
}

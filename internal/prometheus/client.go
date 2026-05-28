package prometheus

import (
	"fmt"
	"net/http"

	prometheusapi "github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// NewAPI creates a Prometheus v1 API client.
// If token is set it adds a Bearer Authorization header.
// If username and password are set it uses HTTP Basic Auth.
func NewAPI(url, token, username, password string) (prometheusv1.API, error) {
	rt := prometheusapi.DefaultRoundTripper
	if token != "" {
		rt = bearerRoundTripper{token: token, next: rt}
	} else if username != "" {
		rt = basicAuthRoundTripper{username: username, password: password, next: rt}
	}

	client, err := prometheusapi.NewClient(prometheusapi.Config{
		Address:      url,
		RoundTripper: rt,
	})
	if err != nil {
		return nil, fmt.Errorf("create prometheus client: %w", err)
	}
	return prometheusv1.NewAPI(client), nil
}

type bearerRoundTripper struct {
	token string
	next  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}

type basicAuthRoundTripper struct {
	username, password string
	next               http.RoundTripper
}

func (b basicAuthRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.SetBasicAuth(b.username, b.password)
	return b.next.RoundTrip(r)
}

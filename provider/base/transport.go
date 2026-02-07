package base

import "net/http"

// ProviderTransportSpec is an optional interface for custom transports.
type ProviderTransportSpec interface {
	http.RoundTripper
}

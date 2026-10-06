package ingress

import "time"

// Domain represents a reverse proxy routing rule.
type Domain struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	ServiceID       string    `json:"serviceId"`
	Hostname        string    `json:"hostname"`
	ContainerPort   int       `json:"containerPort"`
	TLS             bool      `json:"tls"`
	HTTPSRedirect   bool      `json:"httpsRedirect"`
	PathPrefix      string    `json:"pathPrefix"`
	StripPathPrefix bool      `json:"stripPathPrefix"`
	WebSocket       bool      `json:"websocket"`
	CORS            bool      `json:"cors"`
	HSTS            bool      `json:"hsts"`
	BasicAuth       bool      `json:"basicAuth"`
	BasicAuthUser   string    `json:"basicAuthUser,omitempty"`
	BasicAuthPass   string    `json:"basicAuthPass,omitempty"`
	DNSStatus       string    `json:"dnsStatus"`
	CreatedAt       time.Time `json:"createdAt"`
}

// TrafficRequest represents telemetry of an HTTP request proxied through Caddy.
type TrafficRequest struct {
	Timestamp  string `json:"timestamp"`
	ClientIP   string `json:"clientIp"`
	Method     string `json:"method"`
	Host       string `json:"host"`
	URI        string `json:"uri"`
	Status     int    `json:"status"`
	DurationMs int    `json:"durationMs"`
	UserAgent  string `json:"userAgent"`
}

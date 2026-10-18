package ingress

import (
	"strings"
	"time"
)

// Domain represents a reverse proxy routing rule.
type Domain struct {
	ID              string `json:"id"`
	ProjectID       string `json:"projectId"`
	ServiceID       string `json:"serviceId"`
	TargetContainer string `json:"targetContainer,omitempty"`
	Hostname        string `json:"hostname"`
	ContainerPort   int    `json:"containerPort"`
	TLS             bool   `json:"tls"`
	HTTPSRedirect   bool   `json:"httpsRedirect"`
	PathPrefix      string `json:"pathPrefix"`
	StripPathPrefix bool   `json:"stripPathPrefix"`
	WebSocket       bool   `json:"websocket"`
	CORS            bool   `json:"cors"`
	HSTS            bool   `json:"hsts"`
	BasicAuth       bool   `json:"basicAuth"`
	BasicAuthUser   string `json:"basicAuthUser,omitempty"`
	// BasicAuthPass stores the bcrypt hash at rest. It is write-only: accepted on
	// create/update through the request wrapper and never serialized in responses.
	BasicAuthPass string `json:"-"`
	// HasBasicAuth is a computed, response-only flag telling whether a stored
	// password hash exists for the domain.
	HasBasicAuth bool      `json:"hasBasicAuth"`
	DNSStatus    string    `json:"dnsStatus"`
	CreatedAt    time.Time `json:"createdAt"`
}

// computeBasicAuthState fills response-only derived fields from the stored hash.
func (d *Domain) computeBasicAuthState() {
	d.HasBasicAuth = strings.TrimSpace(d.BasicAuthPass) != ""
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

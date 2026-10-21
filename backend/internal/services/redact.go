package services

import "strings"

// Redaction boundary for Service JSON responses.
//
// The repository and WorkloadService keep working with real values (the
// deployer needs them); masking happens only when a Service is about to be
// serialized to an HTTP client. Two assets are affected:
//
//   - secret env values  -> replaced by EnvSecretMask
//   - webhook token      -> hidden; HasWebhook reports whether one exists
//   - webhook secret     -> hidden; HasWebhookSecret reports whether one exists
//
// UpdateService treats EnvSecretMask (and an empty value) for secret vars as
// "keep the existing value", so a client may round-trip a redacted payload
// without destroying secrets.

// RedactService returns a copy of s safe for API serialization.
func RedactService(s Service) Service {
	out := s
	// Preserve flags already resolved by the caller (for example an update
	// response whose token/secret are intentionally omitted from the request).
	out.HasWebhook = s.HasWebhook || s.WebhookToken != ""
	out.WebhookToken = ""
	out.HasWebhookSecret = s.HasWebhookSecret || s.WebhookSecret != "" || s.WebhookSecretError
	out.WebhookSecret = ""
	out.EnvVars = RedactEnvVars(s.EnvVars)
	return out
}

// RedactServices maps RedactService over a slice.
func RedactServices(list []Service) []Service {
	if list == nil {
		return list
	}
	out := make([]Service, len(list))
	for i, s := range list {
		out[i] = RedactService(s)
	}
	return out
}

// RedactEnvVars masks the value of every secret env var that has one. Empty
// secret values stay empty; non-secret vars keep their value.
func RedactEnvVars(vars []EnvVar) []EnvVar {
	if vars == nil {
		return nil
	}
	out := make([]EnvVar, len(vars))
	for i, v := range vars {
		if v.Secret && v.Value != "" {
			v.Value = EnvSecretMask
		}
		out[i] = v
	}
	return out
}

// SecretEnvKeySet returns the keys of env vars flagged secret. Writers of
// deployment logs and unit content use it to mask exact values the API
// redacts, independently of key-name heuristics.
func SecretEnvKeySet(s Service) map[string]bool {
	set := map[string]bool{}
	for _, v := range s.EnvVars {
		if v.Secret && strings.TrimSpace(v.Key) != "" {
			set[v.Key] = true
		}
	}
	return set
}

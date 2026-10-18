package runtime

import "strings"

// envValueMask mirrors services.EnvSecretMask (kept local to avoid a package
// cycle). Quadlet API responses show it in place of encrypted env values.
const envValueMask = "••••••"

// maskEnvironmentValues masks the value of every `Environment=` assignment
// whose key belongs to secretKeys. Unit files keep real values (systemd needs
// them); only the API response is masked. Both the quoted form GOPOD generates
// (`Environment="KEY=VALUE"`) and plain `Environment=KEY=VALUE` are handled,
// including several assignments on one line.
func maskEnvironmentValues(content string, secretKeys map[string]bool) string {
	if content == "" || len(secretKeys) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Environment=") {
			continue
		}
		assignments := strings.TrimPrefix(trimmed, "Environment=")
		masked, didChange := maskEnvAssignments(assignments, secretKeys)
		if !didChange {
			continue
		}
		lines[i] = strings.Replace(line, assignments, masked, 1)
		changed = true
	}
	if !changed {
		return content
	}
	return strings.Join(lines, "\n")
}

// maskEnvAssignments rewrites KEY=VALUE tokens whose key is in secretKeys,
// preserving quoting and the surrounding whitespace.
func maskEnvAssignments(input string, secretKeys map[string]bool) (string, bool) {
	var out strings.Builder
	changed := false
	i := 0
	for i < len(input) {
		if input[i] == ' ' || input[i] == '\t' {
			out.WriteByte(input[i])
			i++
			continue
		}

		start := i
		quoted := input[i] == '"'
		if quoted {
			i++
		}
		var token strings.Builder
		for i < len(input) {
			if quoted {
				if input[i] == '\\' && i+1 < len(input) {
					token.WriteByte(input[i])
					i++
					token.WriteByte(input[i])
					i++
					continue
				}
				if input[i] == '"' {
					i++
					break
				}
			} else if input[i] == ' ' || input[i] == '\t' {
				break
			}
			token.WriteByte(input[i])
			i++
		}

		raw := token.String()
		if eq := strings.Index(raw, "="); eq > 0 && secretKeys[raw[:eq]] {
			masked := raw[:eq+1] + envValueMask
			if quoted {
				out.WriteByte('"')
				out.WriteString(masked)
				out.WriteByte('"')
			} else {
				out.WriteString(masked)
			}
			changed = true
		} else {
			out.WriteString(input[start:i])
		}
	}
	return out.String(), changed
}

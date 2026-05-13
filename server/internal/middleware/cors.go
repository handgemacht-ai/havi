package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowedOrigins string, next http.Handler) http.Handler {
	patterns := parsePatterns(allowedOrigins)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			for _, p := range patterns {
				if matchPattern(p, origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Max-Age", "3600")
					break
				}
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parsePatterns(origins string) []string {
	if origins == "" {
		origins = "chrome-extension://*,moz-extension://*,http://localhost:*"
	}
	parts := strings.Split(origins, ",")
	patterns := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

func matchPattern(pattern, value string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}

	parts := strings.SplitN(pattern, "*", 2)
	prefix := parts[0]
	suffix := parts[1]

	if !strings.HasPrefix(value, prefix) {
		return false
	}
	if !strings.HasSuffix(value, suffix) {
		return false
	}
	return len(value) >= len(prefix)+len(suffix)
}

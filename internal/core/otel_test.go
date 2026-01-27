package core

import (
	"testing"
)

func TestProcessEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		configInsecure bool
		wantEndpoint   string
		wantInsecure   bool
		wantErr        bool
	}{
		{
			name:           "Empty endpoint",
			endpoint:       "",
			configInsecure: false,
			wantEndpoint:   "",
			wantInsecure:   false,
			wantErr:        false,
		},
		{
			name:           "No scheme - host only",
			endpoint:       "otel.jmdt.io:4317",
			configInsecure: false,
			wantEndpoint:   "otel.jmdt.io:4317",
			wantInsecure:   false,
			wantErr:        false,
		},
		{
			name:           "No scheme - explicit insecure config",
			endpoint:       "localhost:4317",
			configInsecure: true,
			wantEndpoint:   "localhost:4317",
			wantInsecure:   true,
			wantErr:        false,
		},
		{
			name:           "HTTPS scheme - secure override",
			endpoint:       "https://otel.jmdt.io",
			configInsecure: true, // Config says insecure, but scheme says secure
			wantEndpoint:   "otel.jmdt.io:443",
			wantInsecure:   false, // Should be false (secure)
			wantErr:        false,
		},
		{
			name:           "HTTPS scheme - with explicit port",
			endpoint:       "https://otel.jmdt.io:8443",
			configInsecure: true,
			wantEndpoint:   "otel.jmdt.io:8443",
			wantInsecure:   false,
			wantErr:        false,
		},
		{
			name:           "HTTP scheme - insecure override",
			endpoint:       "http://localhost",
			configInsecure: false, // Config says secure, but scheme says insecure
			wantEndpoint:   "localhost:80",
			wantInsecure:   true, // Should be true (insecure)
			wantErr:        false,
		},
		{
			name:           "HTTP scheme - with explicit port",
			endpoint:       "http://localhost:4318",
			configInsecure: false,
			wantEndpoint:   "localhost:4318",
			wantInsecure:   true,
			wantErr:        false,
		},
		{
			name:           "Unsupported scheme",
			endpoint:       "ftp://otel.jmdt.io:21",
			configInsecure: false,
			wantEndpoint:   "",
			wantInsecure:   false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEndpoint, gotInsecure, err := processEndpoint(tt.endpoint, tt.configInsecure)

			if tt.wantErr {
				if err == nil {
					t.Errorf("processEndpoint() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("processEndpoint() unexpected error = %v", err)
				return
			}

			if gotEndpoint != tt.wantEndpoint {
				t.Errorf("processEndpoint() gotEndpoint = %v, want %v", gotEndpoint, tt.wantEndpoint)
			}
			if gotInsecure != tt.wantInsecure {
				t.Errorf("processEndpoint() gotInsecure = %v, want %v", gotInsecure, tt.wantInsecure)
			}
		})
	}
}

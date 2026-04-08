package client

import (
	"os"
	"testing"
)

func TestOtelServiceName(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  string
	}{
		{
			name:  "unset uses default",
			unset: true,
			want:  "happeninghound",
		},
		{
			name:  "configured value is used",
			value: "my-service",
			want:  "my-service",
		},
		{
			name:  "whitespace falls back to default",
			value: "   ",
			want:  "happeninghound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				original, hadOriginal := os.LookupEnv("OTEL_SERVICE_NAME")
				t.Cleanup(func() {
					if hadOriginal {
						_ = os.Setenv("OTEL_SERVICE_NAME", original)
						return
					}
					_ = os.Unsetenv("OTEL_SERVICE_NAME")
				})
				_ = os.Unsetenv("OTEL_SERVICE_NAME")
			} else {
				t.Setenv("OTEL_SERVICE_NAME", tt.value)
			}
			if got := otelServiceName(); got != tt.want {
				t.Fatalf("otelServiceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

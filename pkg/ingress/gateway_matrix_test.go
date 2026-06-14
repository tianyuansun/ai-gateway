package ingress

import (
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/translator"
)

func TestResolveTranslatorFullMatrix(t *testing.T) {
	tests := []struct {
		name     string
		exposed  translator.APIFormat
		chat     string
		anthropic string
		responses string
		expectURL string
	}{
		// Passthrough cases (same format)
		{"Chat→Chat passthru", translator.FormatChat, "http://c", "", "", "/chat/completions"},
		{"Anthropic→Anthropic passthru", translator.FormatAnthropic, "", "http://a", "", "/messages"},
		{"Responses→Responses passthru", translator.FormatResponses, "", "", "http://r", "/responses"},

		// Cross-format
		{"Responses→Anthropic", translator.FormatResponses, "", "http://a", "", "/messages"},
		{"Responses→Chat", translator.FormatResponses, "http://c", "", "", "/chat/completions"},
		{"Anthropic→Responses", translator.FormatAnthropic, "", "", "http://r", "/responses"},
		{"Anthropic→Chat", translator.FormatAnthropic, "http://c", "", "", "/chat/completions"},
		{"Chat→Anthropic", translator.FormatChat, "", "http://a", "", "/messages"},
		{"Chat→Responses", translator.FormatChat, "", "", "http://r", "/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &config.Provider{
				Endpoints: config.ProviderEndpoints{
					Chat:      tt.chat,
					Anthropic: tt.anthropic,
					Responses: tt.responses,
				},
			}
			// Need a Gateway instance to call resolveTranslator
			gw := NewGateway(&config.Config{})
			_, gotURL := gw.resolveTranslator(tt.exposed, prov)
			if gotURL != tt.expectURL {
				t.Errorf("expected URL %q, got %q", tt.expectURL, gotURL)
			}
		})
	}
}

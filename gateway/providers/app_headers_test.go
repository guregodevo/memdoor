package providers

import (
	"net/http"
	"strings"
	"testing"
)

func TestOnlyOpenRouterLearnsTheAppsName(t *testing.T) {
	or, _ := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", nil)
	SetAppHeaders(or)
	if or.Header.Get("X-Title") != "Memdoor" || or.Header.Get("HTTP-Referer") != "https://memdoor.ai" {
		t.Fatalf("OpenRouter requests name the app: %v", or.Header)
	}
	other, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	SetAppHeaders(other)
	if other.Header.Get("X-Title") != "" || other.Header.Get("HTTP-Referer") != "" {
		t.Fatalf("a vendor request carries no OpenRouter headers: %v", other.Header)
	}
	if !strings.HasPrefix(other.Header.Get("User-Agent"), "Memdoor/") || !strings.Contains(other.Header.Get("User-Agent"), "memdoor.ai") {
		t.Fatalf("every provider sees the app's name: %q", other.Header.Get("User-Agent"))
	}
	if AppHeaders("https://api.openai.com/v1")["X-Title"] != "" || AppHeaders("https://api.openai.com/v1")["User-Agent"] == "" || AppHeaders("https://openrouter.ai/api/v1/models")["X-Title"] != "Memdoor" {
		t.Fatal("the map form agrees with the request form")
	}
}

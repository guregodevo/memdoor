package providers

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// RunDone tells the company AI gateway a run is over: POST <base>/run/done
// ?run=<id>, the run also in X-Memdoor-Run as on each of its requests. A
// gateway that places runs on boxes (boostr's pool) frees the run's place;
// any other answer, a 404 included, changes nothing. Nothing is sent when no
// company gateway is set, and the call never waits on the gateway.
func RunDone(run string) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("AI_GATEWAY_BASE_URL")), "/")
	if base == "" || run == "" {
		return
	}
	token := gatewayToken()
	go func() {
		if err := postRunDone(base, token, run); err != nil {
			slog.Debug("run done not delivered", slog.String("run", run), slog.String("error", err.Error()))
		}
	}()
}

var runDoneClient = &http.Client{
	Timeout:       5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func postRunDone(base, token, run string) error {
	req, err := http.NewRequest(http.MethodPost, base+"/run/done?run="+url.QueryEscape(run), nil)
	if err != nil {
		return err
	}
	req.Header.Set(attributionHeaderNames()[AttrRun], run)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	setVendorHeaders(req.Header)
	resp, err := runDoneClient.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

package billingsvc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"memdoor/gateway/logs"
)

// Stripe transport for the billing service. Secrets live ONLY in this
// process's environment (STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET) — the
// distributed binary's gateway mode never reads them. No SDK: session
// creation is one form POST and the signature scheme is documented HMAC.

// stripeAPIVersion pins the API surface we tested against.
const stripeAPIVersion = "2024-06-20"

// stripeSecretKey is the key, checked for shape before it goes anywhere.
func stripeSecretKey() (string, error) {
	secretKey := os.Getenv("STRIPE_SECRET_KEY")
	if secretKey == "" {
		return "", fmt.Errorf("STRIPE_SECRET_KEY unset")
	}
	return secretKey, nil
}

// CreateSubscriptionCheckout starts a pro subscription (ADR-0008): one
// price that covers all use, monthly or yearly. The prices live in Stripe;
// their ids come from the environment, so a price change is a dashboard
// edit and a restart, never a deploy.
//
// customerEmail, when known, prefills Stripe's form and pins the receipt to
// the address the app will sign in with.
func CreateSubscriptionCheckout(workspace, period, customerEmail string) (string, string, error) {
	secretKey, err := stripeSecretKey()
	if err != nil {
		return "", "", err
	}
	var price string
	switch period {
	case "monthly":
		price = os.Getenv("STRIPE_PRO_MONTHLY_PRICE")
	case "yearly":
		price = os.Getenv("STRIPE_PRO_YEARLY_PRICE")
	default:
		return "", "", fmt.Errorf("period must be monthly or yearly")
	}
	if price == "" {
		return "", "", fmt.Errorf("no Stripe price configured for the %s pro plan", period)
	}
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("client_reference_id", workspace)
	form.Set("metadata[period]", period)
	form.Set("subscription_data[metadata][workspace]", workspace)
	if customerEmail != "" {
		form.Set("customer_email", customerEmail)
	}
	form.Set("success_url", "https://memdoor.ai/pro/success")
	form.Set("cancel_url", "https://memdoor.ai/pro/cancelled")
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price]", price)
	return postCheckout(secretKey, form)
}

func postCheckout(secretKey string, form url.Values) (string, string, error) {
	req, _ := http.NewRequest(http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if ctx := os.Getenv("STRIPE_CONTEXT"); ctx != "" {
		req.Header.Set("Stripe-Context", ctx)
	}
	// Organization keys also require an explicit API version (live: without
	// it Stripe answers "You did not provide an API version"). Pinning is
	// good practice anyway — the fields we read are stable at this version.
	req.Header.Set("Stripe-Version", stripeAPIVersion)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", "", fmt.Errorf("stripe unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var sess struct {
		ID  string `json:"id"`
		URL string `json:"url"`
		Err *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &sess) != nil || sess.URL == "" {
		if sess.Err != nil {
			return "", "", fmt.Errorf("stripe: %s", sess.Err.Message)
		}
		return "", "", fmt.Errorf("stripe returned no checkout url")
	}
	return sess.URL, sess.ID, nil
}

// verifyStripeSignature checks the Stripe-Signature header against the
// raw payload: t=<unix>,v1=<hmac_sha256(secret, t + "." + payload)>.
// Tolerance bounds replay of captured events.
func verifyStripeSignature(payload []byte, sigHeader, secret string, now time.Time, tolerance time.Duration) error {
	var ts string
	var v1s []string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			v1s = append(v1s, kv[1])
		}
	}
	if ts == "" || len(v1s) == 0 {
		return fmt.Errorf("malformed Stripe-Signature header")
	}
	tsec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("bad timestamp in signature")
	}
	if d := now.Sub(time.Unix(tsec, 0)); d > tolerance || d < -tolerance {
		return fmt.Errorf("signature timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, v1 := range v1s {
		if hmac.Equal([]byte(expected), []byte(v1)) {
			return nil
		}
	}
	return fmt.Errorf("signature mismatch")
}

// applySubscriptionEvent handles the two events a pro subscription sends:
// the checkout that starts it (mode "subscription") and the deletion that
// ends it. Returns whether the event was one of those.
func applySubscriptionEvent(auth *authStore, payload []byte) (bool, string, error) {
	var evt struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID                string            `json:"id"`
				Mode              string            `json:"mode"`
				ClientReferenceID string            `json:"client_reference_id"`
				Subscription      string            `json:"subscription"`
				PaymentStatus     string            `json:"payment_status"`
				Metadata          map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return false, "", fmt.Errorf("unparseable event: %w", err)
	}
	o := evt.Data.Object
	switch {
	case evt.Type == "checkout.session.completed" && o.Mode == "subscription":
		if o.ClientReferenceID == "" || o.Subscription == "" {
			return true, evt.Type, fmt.Errorf("subscription checkout names no workspace or subscription")
		}
		if o.PaymentStatus != "" && o.PaymentStatus != "paid" && o.PaymentStatus != "no_payment_required" {
			return true, evt.Type, fmt.Errorf("subscription checkout %s is %s", o.ID, o.PaymentStatus)
		}
		period := o.Metadata["period"]
		if period == "" {
			period = "monthly"
		}
		auth.SetSubscription(o.ClientReferenceID, o.Subscription, period)
		return true, evt.Type, nil
	case evt.Type == "customer.subscription.deleted":
		if ws := auth.EndSubscription(o.ID); ws == "" {
			return true, evt.Type, fmt.Errorf("subscription %s ended but no workspace held it", o.ID)
		}
		return true, evt.Type, nil
	}
	return false, evt.Type, nil
}

// creditedWorkspace names the workspace a checkout paid for, so the caller can
// promote it. Returned separately from applyCheckoutEvent because promotion is
// an AUTH concern and crediting is a LEDGER one — the webhook is the only place
// that legitimately touches both.
func creditedWorkspace(payload []byte) string {
	var evt struct {
		Data struct {
			Object struct {
				ClientReferenceID string `json:"client_reference_id"`
			} `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &evt) != nil {
		return ""
	}
	return evt.Data.Object.ClientReferenceID
}

// handleStripeWebhook — POST /v1/webhook/stripe. UNAUTHENTICATED route:
// the signature IS the authentication.
func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		writeErr(w, http.StatusNotImplemented, fmt.Errorf("webhook not configured"))
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := verifyStripeSignature(payload, r.Header.Get("Stripe-Signature"), secret, time.Now(), 5*time.Minute); err != nil {
		logs.New("Billing").Warn("webhook signature rejected: " + err.Error())
		writeErr(w, http.StatusBadRequest, fmt.Errorf("bad signature"))
		return
	}
	if handled, evtType, serr := applySubscriptionEvent(s.auth, payload); handled {
		if serr != nil {
			logs.New("Billing").Warn(fmt.Sprintf("subscription event not applied (%s): %v", evtType, serr))
		} else {
			logs.New("Billing").Info("subscription event applied (" + evtType + ")")
			if evtType == "checkout.session.completed" {
				s.welcomeAfterSubscription(creditedWorkspace(payload))
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	// A one-off payment (the old prepaid credits) buys nothing any more:
	// the product is the subscription, handled above.
	w.WriteHeader(http.StatusOK)
}

// handleSubscribe — POST /v1/subscribe {period: monthly|yearly}. The pro
// plan's payment link, for the signed-in workspace.
func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
		return
	}
	ws, err := s.authWorkspace(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err)
		return
	}
	var req struct {
		Period string `json:"period"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Period == "" {
		req.Period = "monthly"
	}
	link, id, err := CreateSubscriptionCheckout(ws, req.Period, s.auth.emailFor(ws))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	logs.New("Billing").Info("subscription checkout " + id + " for " + ws + " (" + req.Period + ")")
	writeJSON(w, map[string]string{"url": link, "session_id": id, "period": req.Period})
}

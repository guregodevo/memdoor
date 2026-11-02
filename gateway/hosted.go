package gateway

import (
	"net/http"
	"os"
	"strings"
)

// MEMDOOR_HOSTED=off turns off the services memdoor.ai hosts on a gateway:
// sign-in and accounts over the API, the relay for a gateway kept to
// localhost, /share. Every one of their routes answers 410 with one line, the
// site hides its sign-in, and the billing unit stays stopped (the deploy reads
// the same switch). The default is on. A gateway on a person's own machine
// never sets it: its accounts, relay and shares are its own.
func hostedOff() bool {
	v := strings.TrimSpace(os.Getenv("MEMDOOR_HOSTED"))
	return v == "off" || v == "0"
}

const hostedOffMessage = `{"error":"sign-in on memdoor.ai is off"}`

// hostedOr serves h, or 410 when the hosted services are off.
func hostedOr(h http.Handler) http.Handler {
	if !hostedOff() {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(hostedOffMessage))
	})
}

// hostedMeta is what the site reads to hide its sign-in: one tag in the head
// of every served page while the hosted services are off.
const hostedMeta = `<meta name="memdoor-hosted" content="off">`

func withHostedMeta(index []byte) []byte {
	if !hostedOff() {
		return index
	}
	return []byte(strings.Replace(string(index), "</head>", hostedMeta+"</head>", 1))
}

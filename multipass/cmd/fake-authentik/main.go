// Command fake-authentik serves internal/fakeauthentik's test members as an
// Authentik API, so multipass runs locally without a real Authentik.
// Development only.
//
//	go run ./cmd/fake-authentik            # listens on 127.0.0.1:9100
package main

import (
	"flag"
	"log"
	"net/http"

	"multipass/internal/fakeauthentik"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9100", "listen address")
	flag.Parse()
	log.Printf("fake Authentik on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, fakeauthentik.Handler()))
}

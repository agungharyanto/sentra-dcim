// Command sentra is the Sentra DCIM backend.
//
// Infra placeholder (v0.1.2): serves a liveness endpoint so the container stack can be
// validated. The real backend skeleton lands in v0.1.3.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "serve":
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		srv := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "healthcheck":
		c := http.Client{Timeout: 2 * time.Second}
		resp, err := c.Get("http://127.0.0.1:8080/api/v1/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		os.Exit(2)
	}
}

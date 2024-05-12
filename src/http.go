package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

func Cors(fs http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")
		fs.ServeHTTP(w, r)
	}
}

func ServeHTTP(staticDir string) {
	var (
		err     error
		addr    *string = flag.String("addr", ":8080", "the address to listen on, default :8080")
		server  http.Server
		handler *http.ServeMux
		limiter *BandwidthLimiter
	)
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	handler = &http.ServeMux{}

	if limiter, err = NewBandwidthLimiter(); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}


	// static file host
	handler.Handle("/", Cors(http.FileServer(http.Dir(staticDir))))

	handler.HandleFunc("/bandwidth/{method}", func(w http.ResponseWriter, r *http.Request) {
		var (
			method string = r.PathValue("method")
		)
		log.Printf("%s %s\n", r.Method, r.URL.Path)

		switch r.Method {
		case "GET":
			switch method {
			case "get":
				// Case: GET /bandwidth/get -> responds with { "currentBandwidth": int64 }
				var (
					report BandwidthReport = limiter.GetCurrentBandwidth()
					buf    []byte
				)

				if buf, err = report.Encode(); err != nil {
					log.Printf("Error: %s\n", err)
					return
				}

				if _, err = w.Write(buf); err != nil {
					log.Printf("Error: %s\n", err)
					return
				}
			default:
				w.WriteHeader(500)
			}
		case "POST":
			switch method {
			case "set":
				// Case: POST /bandwidth/set with Body: [{ "speed": int64, "duration": uint64, "latency": uint64 }] parsed to ``[]Trajectory`` -> responds with 200 immediately
				var (
					buf        []byte
					trajectory []Trajectory
				)

				if buf, err = readToEOF(r.Body); err != nil {
					log.Printf("Error: %s\n", err)
					return
				}

				if err = json.Unmarshal(buf, &trajectory); err != nil {
					log.Printf("Error: %s\n", err)
					return
				}

				go limiter.SetBandwidth(trajectory)
				w.WriteHeader(200)
			case "reset":
				// Case: POST /bandwidth/reset -> responds with 200 immediately
				limiter.DeleteBandwidth()
			default:
				w.WriteHeader(500)
			}
		default:
			w.WriteHeader(500)
		}
	})

	server = http.Server{
		Addr:    *addr,
		Handler: handler,
	}

	log.Printf("Listening on http://localhost%s/\n", *addr)
	log.Printf("MoQ Encoder running on http://localhost%s/src-encoder/\n", *addr)
	log.Printf("MoQ Player running on http://localhost%s/src-player/\n", *addr)
	if err = server.ListenAndServe(); err != nil {
		log.Panicf("ErrServe: %s\n", err)
	}
}

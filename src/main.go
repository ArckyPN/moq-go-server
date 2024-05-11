/*
Copyright (c) Meta Platforms, Inc. and affiliates.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package main

import (
	"context"
	"crypto/tls"
	"facebookexperimental/moq-go-server/moqconnectionmanagment"
	"facebookexperimental/moq-go-server/moqfwdtable"
	"facebookexperimental/moq-go-server/moqmessageobjects"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/logging"
	"github.com/quic-go/quic-go/qlog"
	"github.com/quic-go/webtransport-go"

	log "github.com/sirupsen/logrus"
)

// Default parameters
const HTTP_SERVER_LISTEN_ADDR = ":4433"
const TLS_CERT_FILEPATH = "../../cert/localhost.pem"
const TLS_KEY_FILEPATH = "../../cert/localhost-key.pem"
const OBJECT_EXPIRATION_MS = 3 * 60 * 1000
const CACHE_CLEAN_UP_PERIOD_MS = 10 * 1000
const HTTP_CONNECTION_KEEP_ALIVE_MS = 10 * 1000

func main() {
	// Parse params
	listenAddr := flag.String("listen_addr", HTTP_SERVER_LISTEN_ADDR, "Server listen port (example: \":4433\")")
	tlsCertPath := flag.String("tls_cert", TLS_CERT_FILEPATH, "TLS certificate file path to use in this server")
	tlsKeyPath := flag.String("tls_key", TLS_KEY_FILEPATH, "TLS key file path to use in this server")
	objExpMs := flag.Uint64("obj_exp_ms", OBJECT_EXPIRATION_MS, "Object TTL in this server (in milliseconds)")
	cacheCleanUpPeriodMs := flag.Uint64("cache_cleanup_period_ms", CACHE_CLEAN_UP_PERIOD_MS, "Execute clean up task every (in milliseconds)")
	httpConnTimeoutMs := flag.Uint64("http_conn_time_out_ms", HTTP_CONNECTION_KEEP_ALIVE_MS, "HTTP connection timeout (in milliseconds)")

	var (
		err     error
		tlsCert tls.Certificate
	)

	flag.Parse()

	log.SetFormatter(&log.TextFormatter{})

	if err = clearQlogDirectory(); err != nil {
		log.Error(fmt.Sprintf("qlog dir: %s\n", err))
		return
	}

	if tlsCert, err = tls.LoadX509KeyPair(*tlsCertPath, *tlsKeyPath); err != nil {
		log.Error(fmt.Sprintf("tls: %s\n", err))
		return
	}

	// Create moqt obj forward table
	moqtFwdTable := moqfwdtable.New()

	// create objects mem storage (relay)
	objects := moqmessageobjects.New(*cacheCleanUpPeriodMs)

	server := &webtransport.Server{
		H3: http3.Server{
			Addr: *listenAddr,
			QUICConfig: &quic.Config{
				Tracer: func(ctx context.Context, p logging.Perspective, ci quic.ConnectionID) *logging.ConnectionTracer {
					var (
						e    error
						path string = fmt.Sprintf("%s/data/qlog/%s-%s.qlog", cwd(), p, ci.String())
						fp   *os.File
					)
					if fp, e = createFile(path); e != nil {
						log.Error(fmt.Sprintf("qlog: %s\n", e))
						panic(e)
					}

					return qlog.NewConnectionTracer(fp, p, ci)
				},
				MaxIdleTimeout: time.Duration(*httpConnTimeoutMs) * time.Millisecond,
			},
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
			},
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/moq", func(rw http.ResponseWriter, r *http.Request) {
		var (
			session *webtransport.Session
		)

		if session, err = server.Upgrade(rw, r); err != nil {
			log.Error(fmt.Sprintf("tls: %s\n", err))
			return
		}

		namespace := r.URL.Path
		log.Info(fmt.Sprintf("%s - Accepted incoming WebTransport session. rawQuery: %s", namespace, r.URL.RawQuery))

		moqconnectionmanagment.MoqConnectionManagment(session, namespace, moqtFwdTable, objects, *objExpMs)
	})

	log.Info("Launching WebTransport server at: ", server.H3.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Error(fmt.Sprintf("Server error: %s", err))
		return
	}

	objects.Stop()
}

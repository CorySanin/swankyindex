// swankyindex - directory index listing with download counts
// Copyright (C) 2026  Cory Sanin

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CorySanin/swankyindex/internal/config"
	"github.com/CorySanin/swankyindex/internal/web"
	"github.com/CorySanin/swankyindex/pkg/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conf := config.Config()
	if err := os.MkdirAll(filepath.Dir(*conf.Storage), 0755); err != nil {
		log.Fatalf("failed to create storage directory: %v", err)
	}
	store := storage.New(*conf.Storage)
	var wg sync.WaitGroup

	server := web.NewServer(&conf, &store, &wg)

	mux := http.NewServeMux()
	mux.Handle("/.static/", http.StripPrefix("/.static", notFoundOnDir(http.FileServer(http.Dir("./static")))))
	mux.HandleFunc(web.ApiPath, server.ApiHandler)
	mux.HandleFunc("/", server.Handler)
	var metricsServer *http.Server = nil
	if *conf.PrometheusPort == 0 || *conf.PrometheusPort == *conf.Port {
		mux.Handle(*conf.PrometheusPath, server.MetricsHandler())
	} else if *conf.PrometheusPort > 0 {
		metricsServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", *conf.PrometheusPort),
			Handler: server.MetricsHandler(),
		}
		go func() {
			fmt.Printf("Prometheus client listening on port %d\n", *conf.PrometheusPort)
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("listen: %s\n", err)
			}
		}()
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", *conf.Port),
		Handler: mux,
	}

	go func() {
		fmt.Printf("Listening on port %d\n", *conf.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Metrics server forced to shutdown: %v", err)
		}
	}

	wg.Wait()
	store.Optimize()
}

func notFoundOnDir(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

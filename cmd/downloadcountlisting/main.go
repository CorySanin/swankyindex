// downloadcountlisting - directory index listing with download counts
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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/CorySanin/downloadcountlisting/internal/config"
	"github.com/CorySanin/downloadcountlisting/internal/web"
	"github.com/CorySanin/downloadcountlisting/pkg/storage"
)

var wg sync.WaitGroup

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	conf := config.Config()
	os.MkdirAll(filepath.Dir(*conf.Storage), 0755)
	store := storage.New(*conf.Storage)
	web.InitWeb(&conf, &store, &wg)
	server := http.Server{
		Addr: fmt.Sprintf(":%d", *conf.Port),
	}
	http.Handle("/.static/", http.StripPrefix("/.static", notFoundOnDir(http.FileServer(http.Dir("./static")))))
	http.HandleFunc("/.api/", web.ApiHandler)
	http.HandleFunc("/", web.Handler)
	fmt.Printf("Listening on port %d", *conf.Port)
	go server.ListenAndServe()
	<-ctx.Done()
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("could not shutdown server: %v", err)
	}
	wg.Wait()
	store.Optimize()
	os.Exit(0)
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

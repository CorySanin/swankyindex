package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/CorySanin/downloadcountlisting/internal/config"
	"github.com/CorySanin/downloadcountlisting/internal/web"
	"github.com/CorySanin/downloadcountlisting/pkg/storage"
)

func main() {
	conf := config.Config()
	os.MkdirAll(filepath.Dir(*conf.Storage), 0755)
	storage := storage.New(*conf.Storage)
	web.InitWeb(conf, storage)
	http.Handle("/.static/", http.StripPrefix("/.static", notFoundOnDir(http.FileServer(http.Dir("./static")))))
	http.HandleFunc("/.api/", web.ApiHandler)
	http.HandleFunc("/", web.Handler)
	fmt.Printf("Listening on port %d", *conf.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *conf.Port), nil))
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

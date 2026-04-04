package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CorySanin/downloadcountlisting/internal/config"
	"github.com/CorySanin/downloadcountlisting/pkg/storage"
	"github.com/dustin/go-humanize"
)

type (
	FileEntry struct {
		Filename string
		Size     string
		Date     time.Time
		DL       int
		DLTotal  int
	}

	ListingData struct {
		Path           string
		Subdirectories []string
		Files          []FileEntry
		HideDownloads  bool
		Heading        template.HTML
		Footer         template.HTML
	}
)

//go:embed templates/*
var templateFS embed.FS
var conf config.Conf
var templates *template.Template
var store storage.Storage

func InitWeb(cfg config.Conf, st storage.Storage) {
	conf = cfg
	store = st
	mytemplates := []string{"layout.html"}
	for i, v := range mytemplates {
		mytemplates[i] = path.Join("templates", v)
	}
	tmpl, err := template.ParseFS(templateFS, mytemplates...)
	if err != nil {
		log.Fatal(err)
	}
	templates = tmpl
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		apiHandler(w, r)
		return
	}
	destination := path.Join(*conf.Directory, r.URL.Path[1:])
	if childDirs, childFiles, err := getChildren(destination); err == nil {
		var data = ListingData{
			Path:           r.URL.Path,
			Subdirectories: childDirs,
			Files:          childFiles,
			HideDownloads:  *conf.HideDownloads,
			Heading:        conf.Heading,
			Footer:         conf.Footer,
		}
		if err := templates.ExecuteTemplate(w, "layout.html", data); err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			log.Default().Print(err.Error())
		}

		return
	} else if file, err := os.Open(destination); err == nil {
		defer file.Close()
		fp, fileName := filepath.Split(destination)
		fileStat, err := file.Stat()
		if err != nil {
			http.Error(w, "Internal server error.", 500)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Header().Set("Content-Type", r.Header.Get("Content-Type")) // TODO: set content-type accordingly
		w.Header().Set("Content-Length", strconv.FormatInt(fileStat.Size(), 10))

		rec := &responseWriterWithStatus{ResponseWriter: w}
		http.ServeContent(rec, r, fileName, fileStat.ModTime(), file)
		if rec.success() {
			store.IncrementDownload(storage.Download{
				Path:     fp,
				Filename: fileName,
			})
		}
		return
	}
	http.Error(w, "404 file not found", 404)
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	// rpath = r.URL.Path[len("/api/v1/"):]
	http.Error(w, "API not yet implemented", 404)
}

func getChildren(path string) ([]string, []FileEntry, error) {
	ch := make(chan map[string]storage.Totals)
	if !*conf.HideDownloads {
		go store.GetTotalsByPath(path, ch)
	}
	entires, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	var dirCount = 0
	var fileCount = 0
	for _, v := range entires {
		if v.IsDir() {
			dirCount++
		} else {
			fileCount++
		}
	}

	childDirs := make([]string, dirCount)
	childFiles := make([]FileEntry, fileCount)
	dirCount = 0
	fileCount = 0
	var totalsMap map[string]storage.Totals = nil
	if !*conf.HideDownloads {
		totalsMap = <-ch
	}

	for _, v := range entires {
		if v.IsDir() {
			childDirs[dirCount] = v.Name()
			dirCount++
		} else {
			var fEntry = FileEntry{
				Filename: v.Name(),
			}
			if info, err := v.Info(); err == nil {
				fEntry.Size = humanize.IBytes(uint64(info.Size()))
				fEntry.Date = info.ModTime()
			}
			if totalsMap != nil {
				if t, ok := totalsMap[v.Name()]; ok {
					fEntry.DL = t.Recent
					fEntry.DLTotal = t.All
				}
			}
			childFiles[fileCount] = fEntry
			fileCount++
		}
	}
	return childDirs, childFiles, nil
}

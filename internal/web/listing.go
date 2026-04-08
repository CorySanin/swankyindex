package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"maps"
	"mime"
	"net/http"
	"os"
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
		Filename string `json:"filename"`
		Size     string `json:"size"`
		Date     string `json:"date"`
		Time     string `json:"time"`
		DL       int    `json:"dl"`
		DLTotal  int    `json:"dlTotal"`
	}

	ApiErrorResponse struct {
		Error *string `json:"error"`
	}

	ApiListingData struct {
		ApiErrorResponse
		Path           string      `json:"path"`
		Subdirectories []string    `json:"subdirectories"`
		Files          []FileEntry `json:"files"`
	}

	ListingData struct {
		ApiListingData
		Title         string
		Icons         bool
		HideDownloads bool
		Styles        *string
		Heading       template.HTML
		Footer        template.HTML
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
		mytemplates[i] = filepath.Join("templates", v)
	}
	tmpl, err := template.ParseFS(templateFS, mytemplates...)
	if err != nil {
		log.Fatal(err)
	}
	templates = tmpl
}

func Handler(w http.ResponseWriter, r *http.Request) {
	cDir, err := conf.GetDirectory()
	if err != nil {
		log.Fatalf("Get directory failed: %v", err)
	}
	destination := filepath.Join(cDir, r.URL.Path[1:])
	if !strings.HasPrefix(destination+config.SeparatorString, cDir) {
		http.Error(w, "Something went wrong", http.StatusBadRequest)
		log.Printf("User tried accessing %s but was denied.", destination)
		return
	}
	if childDirs, childFiles, ch, err := getChildren(*config.NormalizePath(destination), r.URL.Path != "/"); err == nil {
		defer func() {
			<-ch
		}()
		normalizedDirname := *config.NormalizePath(r.URL.Path)
		var data = ListingData{
			ApiListingData: ApiListingData{
				Path:           normalizedDirname,
				Subdirectories: childDirs,
				Files:          childFiles,
			},
			Title:         *conf.Title,
			Icons:         *conf.Icons,
			HideDownloads: *conf.HideDownloads,
			Styles:        conf.Styles,
			Heading:       template.HTML(strings.ReplaceAll(*conf.Heading, "%path%", template.HTMLEscapeString(normalizedDirname))),
			Footer:        template.HTML(strings.ReplaceAll(*conf.Footer, "%path%", template.HTMLEscapeString(normalizedDirname))),
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
		mt, err := getMimeType(fileName)
		if err != nil {
			mt = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mt)
		w.Header().Set("Content-Disposition", "filename="+fileName)
		w.Header().Set("Content-Length", strconv.FormatInt(fileStat.Size(), 10))

		rec := &responseWriterWithStatus{ResponseWriter: w}
		http.ServeContent(rec, r, fileName, fileStat.ModTime(), file)
		if rec.success() {
			store.IncrementDownload(storage.Download{
				DownloadIndex: storage.DownloadIndex{
					Path:     fp,
					Filename: fileName,
				},
				AccessDomain: r.Host,
				UserAgent:    r.UserAgent(),
			})
		}
		return
	} else if filepath.Base(destination) == "favicon.ico" {
		if file, err := os.Open(filepath.Join("static", "images", filepath.Base(destination))); err == nil {
			defer file.Close()
			_, fileName := filepath.Split(destination)
			fileStat, err := file.Stat()
			if err != nil {
				http.Error(w, "Internal server error.", 500)
				return
			}
			mt, err := getMimeType(fileName)
			if err != nil {
				mt = "application/octet-stream"
			}
			w.Header().Set("Content-Type", mt)
			w.Header().Set("Content-Disposition", "filename="+fileName)
			w.Header().Set("Content-Length", strconv.FormatInt(fileStat.Size(), 10))

			http.ServeContent(w, r, fileName, fileStat.ModTime(), file)
			return
		}
	}
	http.Error(w, "404 file not found", 404)
}

func ApiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	version := r.Header.Get("X-API-Version")
	if version != "1" {
		w.WriteHeader(http.StatusPreconditionFailed)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("unexpected API version")),
		})
		return
	}
	cDir, err := conf.GetDirectory()
	if err != nil {
		log.Fatalf("Get directory failed: %v", err)
	}
	rpath := config.NormalizePath(r.URL.Path[len("/.api/"):])
	destination := filepath.Join(cDir, *rpath)
	if !strings.HasPrefix(destination+config.SeparatorString, cDir) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("bad request")),
		})
		log.Printf("User tried accessing %s via API but was denied.", destination)
		return
	}
	if r.Method != "GET" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("request method must be GET")),
		})
		return
	}
	if childDirs, childFiles, ch, err := getChildren(*config.NormalizePath(destination), *rpath != "/"); err == nil {
		defer func() {
			<-ch
		}()
		var data = ApiListingData{
			Path:           *config.NormalizePath(*rpath),
			Subdirectories: childDirs,
			Files:          childFiles,
		}
		json.NewEncoder(w).Encode(data)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(ApiErrorResponse{
		Error: new(string("not found")),
	})
}

func getMimeType(f string) (string, error) {
	lastDot := strings.LastIndex(f, ".")
	if lastDot < 0 {
		return "", fmt.Errorf("No dot character in %s", f)
	}
	ext := f[lastDot:]
	return mime.TypeByExtension(ext), nil
}

func getChildren(path string, hasParent bool) ([]string, []FileEntry, chan int, error) {
	ch := make(chan map[string]storage.Totals)
	go store.GetTotalsByPath(path, ch)
	entires, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, nil, err
	}
	dirCount := 0
	if hasParent {
		dirCount = 1
	}
	fileCount := 0
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
	totalsMap := <-ch

	if hasParent {
		childDirs[dirCount] = ".."
		dirCount++
	}

	for _, v := range entires {
		if v.IsDir() || isSymlinkDir(path, v) {
			childDirs[dirCount] = v.Name()
			dirCount++
		} else {
			var fEntry = FileEntry{
				Filename: v.Name(),
			}
			if info, err := v.Info(); err == nil {
				fEntry.Size = humanize.IBytes(uint64(info.Size()))
				modtime := info.ModTime()
				fEntry.Date = modtime.Format(time.DateOnly)
				fEntry.Time = modtime.Format("15:04:05 -0700")
			}
			if !*conf.HideDownloads && totalsMap != nil {
				if t, ok := totalsMap[v.Name()]; ok {
					fEntry.DL = t.Recent
					fEntry.DLTotal = t.All
				}
			}
			childFiles[fileCount] = fEntry
			fileCount++
		}
	}
	cleanup := make(chan int)
	go cleanUpRemovedFiles(path, childFiles, totalsMap, cleanup)
	return childDirs, childFiles, cleanup, nil
}

func isSymlinkDir(path string, v os.DirEntry) bool {
	if l, err := filepath.EvalSymlinks(filepath.Join(path, v.Name())); err == nil {
		_, err := os.ReadDir(l)
		return err == nil
	}
	return false,
}

func cleanUpRemovedFiles(p string, childFiles []FileEntry, totals map[string]storage.Totals, ch chan int) {
	for _, v := range childFiles {
		maps.DeleteFunc(totals, func(key string, value storage.Totals) bool {
			return v.Filename == key
		})
	}
	dls := make([]storage.DownloadIndex, 0, len(totals))
	for k := range totals {
		log.Printf("No longer exists: %s", filepath.Join(p, k))
		dls = append(dls, storage.DownloadIndex{
			Path:     p,
			Filename: k,
		})
	}
	if err := store.RemoveDownloads(dls); err != nil {
		log.Printf("Failed removing %d rows: %v", len(dls), err)
		ch <- -1
		return
	}
	if len(dls) > 0 {
		log.Printf("Removed %d rows", len(dls))
	}
	ch <- len(dls)
}

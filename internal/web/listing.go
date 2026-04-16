package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
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

	ListingData struct {
		ApiListingData
		Title         string
		Icons         bool
		HideDownloads bool
		EnableJS      bool
		Styles        *string
		Heading       template.HTML
		Footer        template.HTML
	}

	Server struct {
		conf      *config.Conf
		store     *storage.Storage
		wg        *sync.WaitGroup
		templates *template.Template
	}
)

//go:embed templates/*
var templateFS embed.FS

func NewServer(cfg *config.Conf, st *storage.Storage, waitGroup *sync.WaitGroup) *Server {
	mytemplates := []string{"layout.html"}
	for i, v := range mytemplates {
		mytemplates[i] = filepath.Join("templates", v)
	}
	tmpl, err := template.ParseFS(templateFS, mytemplates...)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	return &Server{
		conf:      cfg,
		store:     st,
		wg:        waitGroup,
		templates: tmpl,
	}
}

func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
	cDir, err := s.conf.GetDirectory()
	if err != nil {
		log.Fatalf("Get directory failed: %v", err)
	}
	destination := filepath.Join(cDir, r.URL.Path[1:])
	if !strings.HasPrefix(filepath.Clean(destination)+config.SeparatorString, cDir) {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Printf("User tried accessing %s but was denied.", destination)
		return
	}
	if childDirs, childFiles, ch, err := s.getChildren(*config.NormalizePath(destination), r.URL.Path); err == nil {
		defer func() {
			<-ch
		}()
		normalizedDirname := *config.NormalizePath(r.URL.Path)
		pathSub := fmt.Sprintf("<span class=\"pathname\">%s</span>", template.HTMLEscapeString(normalizedDirname))
		var data = ListingData{
			ApiListingData: ApiListingData{
				Path:           normalizedDirname,
				Subdirectories: childDirs,
				Files:          childFiles,
			},
			Title:         *s.conf.Title,
			Icons:         *s.conf.Icons,
			HideDownloads: *s.conf.HideDownloads,
			EnableJS:      *s.conf.EnableJS,
			Styles:        s.conf.Styles,
			Heading:       template.HTML(strings.ReplaceAll(*s.conf.Heading, "%path%", pathSub)),
			Footer:        template.HTML(strings.ReplaceAll(*s.conf.Footer, "%path%", pathSub)),
		}
		if err := s.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			log.Printf("Template execution error: %v", err)
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

		mt := mime.TypeByExtension(filepath.Ext(fileName))
		if mt == "" {
			mt = "application/octet-stream"
		}
		rec := &responseWriterWithStatus{ResponseWriter: w}
		rec.Header().Set("Content-Type", mt)
		rec.Header().Set("Content-Disposition", "filename="+fileName)
		rec.Header().Set("Content-Length", strconv.FormatInt(fileStat.Size(), 10))

		http.ServeContent(rec, r, fileName, fileStat.ModTime(), file)
		if rec.success() {
			s.store.IncrementDownload(storage.Download{
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
		if file, err := os.Open(filepath.Join("static", "images", "favicon.ico")); err == nil {
			defer file.Close()
			fileStat, err := file.Stat()
			if err != nil {
				http.Error(w, "Internal server error.", 500)
				return
			}
			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(destination)))
			w.Header().Set("Content-Length", strconv.FormatInt(fileStat.Size(), 10))
			http.ServeContent(w, r, "favicon.ico", fileStat.ModTime(), file)
			return
		}
	}
	http.Error(w, "404 file not found", 404)
}

func (s *Server) getChildren(path string, reqpath string) ([]string, []FileEntry, chan int, error) {
	ch := make(chan map[string]storage.Totals, 1)
	s.wg.Go(func() {
		s.store.GetTotalsByPath(path, ch)
	})

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, nil, err
	}

	if *s.conf.HideDotfiles {
		entries = slices.DeleteFunc(entries, func(ent os.DirEntry) bool {
			return len(ent.Name()) > 0 && ent.Name()[0] == '.'
		})
	}

	if *s.conf.HideSymlinks {
		entries = slices.DeleteFunc(entries, func(ent os.DirEntry) bool {
			info, err := ent.Info()
			if err != nil {
				return true
			}
			return info.Mode()&os.ModeSymlink != 0
		})
	}

	if s.conf.Ignore != nil {
		for _, r := range s.conf.Ignore {
			entries = slices.DeleteFunc(entries, func(ent os.DirEntry) bool {
				return r.MatchString(filepath.Join(reqpath, ent.Name()))
			})
		}
	}

	dirCount := 0
	fileCount := 0
	hasParent := reqpath != "/" && reqpath != ""
	if hasParent {
		dirCount = 1
	}

	for _, v := range entries {
		if isDir(path, v) {
			dirCount++
		} else {
			fileCount++
		}
	}

	childDirs := make([]string, 0, dirCount)
	childFiles := make([]FileEntry, 0, fileCount)

	if hasParent {
		childDirs = append(childDirs, "..")
	}

	totalsMap := <-ch

	for _, v := range entries {
		if isDir(path, v) {
			childDirs = append(childDirs, v.Name())
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
			if !*s.conf.HideDownloads && totalsMap != nil {
				if t, ok := totalsMap[v.Name()]; ok {
					fEntry.DL = t.Recent
					fEntry.DLTotal = t.All
				}
			}
			childFiles = append(childFiles, fEntry)
		}
	}

	cleanup := make(chan int, 1)
	s.wg.Go(func() {
		s.cleanUpRemovedFiles(path, childFiles, totalsMap, cleanup)
	})

	return childDirs, childFiles, cleanup, nil
}

func isDir(path string, v os.DirEntry) bool {
	if v.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(path, v.Name()))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (s *Server) cleanUpRemovedFiles(p string, childFiles []FileEntry, totals map[string]storage.Totals, ch chan int) {
	if len(totals) == 0 {
		ch <- 0
		return
	}
	for _, v := range childFiles {
		delete(totals, v.Filename)
	}
	if len(totals) == 0 {
		ch <- 0
		return
	}

	dls := make([]storage.DownloadIndex, 0, len(totals))
	for k := range totals {
		log.Printf("No longer exists: %s", filepath.Join(p, k))
		dls = append(dls, storage.DownloadIndex{
			Path:     p,
			Filename: k,
		})
	}
	if err := s.store.RemoveDownloads(dls); err != nil {
		log.Printf("Failed removing %d rows: %v", len(dls), err)
		ch <- -1
		return
	}
	log.Printf("Removed %d rows", len(dls))
	ch <- len(dls)
}

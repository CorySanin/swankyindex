package web

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/CorySanin/swankyindex/internal/config"
	"github.com/CorySanin/swankyindex/pkg/storage"
)

type (
	ApiErrorResponse struct {
		Error *string `json:"error"`
	}

	ApiListingData struct {
		ApiErrorResponse
		Path           string      `json:"path"`
		Subdirectories []string    `json:"subdirectories"`
		Files          []FileEntry `json:"files"`
	}

	zipRequest struct {
		Directory string   `json:"directory"`
		Files     []string `json:"files"`
	}
)

const (
	ApiPath    string = "/.api/"
	apiDirPath string = ApiPath + "dir/"
	apiZipPath string = ApiPath + "zip"
)

func (s *Server) ApiHandler(w http.ResponseWriter, r *http.Request) {
	apiVer := r.Header.Get("X-API-Version")
	if apiVer != "1" {
		w.WriteHeader(http.StatusPreconditionFailed)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("unexpected API version")),
		})
		return
	}
	if strings.HasPrefix(r.URL.Path, apiDirPath) {
		s.apiDirHandler(w, r, apiVer)
		return
	} else if r.URL.Path == apiZipPath && *s.conf.EnableZipDownloads {
		s.apiZipHandler(w, r, apiVer)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(ApiErrorResponse{
		Error: new(string("not found")),
	})
}

func (s *Server) apiDirHandler(w http.ResponseWriter, r *http.Request, _ string) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("request method must be GET")),
		})
		return
	}

	cDir, err := s.conf.GetDirectory()
	if err != nil {
		log.Fatalf("Get directory failed: %v", err)
	}
	rpath := config.NormalizePath(r.URL.Path[len(apiDirPath)-1:])
	destination := filepath.Join(cDir, *rpath)
	fp := filepath.Clean(destination) + config.SeparatorString
	if !strings.HasPrefix(fp, cDir) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("bad request")),
		})
		log.Printf("User tried accessing %s via API but was denied.", fp)
		return
	}
	if childDirs, childFiles, ch, err := s.getChildren(fp, *rpath); err == nil {
		defer func() {
			<-ch
		}()
		var data = ApiListingData{
			Path:           *rpath,
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

func (s *Server) apiZipHandler(w http.ResponseWriter, r *http.Request, _ string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("request method must be POST")),
		})
		return
	}
	cDir, err := s.conf.GetDirectory()
	if err != nil {
		log.Fatalf("Get directory failed: %v", err)
	}
	var body zipRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("bad request")),
		})
		return
	}
	rpath := config.NormalizePath(body.Directory)
	destination := filepath.Join(cDir, *rpath)
	fp := filepath.Clean(destination) + config.SeparatorString
	if !strings.HasPrefix(fp, cDir) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("bad request")),
		})
		log.Printf("User tried accessing %s via API but was denied.", destination)
		return
	}
	for _, v := range body.Files {
		if strings.Contains(v, "/") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ApiErrorResponse{
				Error: new(string("bad request")),
			})
			return
		}
	}
	rec := &responseWriterWithStatus{ResponseWriter: w}
	rec.Header().Set("Content-Type", "application/octet-stream")
	rec.Header().Set("Content-Disposition", fmt.Sprintf("filename=%s", zipFilename(destination)))
	zipw := zip.NewWriter(rec)
	defer zipw.Close()
	for _, v := range body.Files {
		if err := zipFile(zipw, path.Join(destination, v)); err != nil {
			return
		}
	}
	zipw.Close()

	if !rec.success() {
		return
	}
	for _, v := range body.Files {
		s.store.IncrementDownload(storage.Download{
			DownloadIndex: storage.DownloadIndex{
				Path:     fp,
				Filename: v,
			},
			AccessDomain: r.Host,
			UserAgent:    r.UserAgent(),
		})
	}
}

func zipFilename(rpath string) string {
	name := filepath.Base(rpath)
	if name == "/" {
		name = "root"
	}
	return fmt.Sprintf("%s.zip", name)
}

func zipFile(zw *zip.Writer, fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(filepath.Base(fname))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

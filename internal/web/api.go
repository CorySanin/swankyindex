package web

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/CorySanin/downloadcountlisting/internal/config"
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
)

const (
	ApiPath    string = "/.api/"
	apiDirPath string = ApiPath + "dir/"
)

func (s *Server) ApiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(ApiErrorResponse{
		Error: new(string("not found")),
	})
}

func (s *Server) apiDirHandler(w http.ResponseWriter, r *http.Request, apiVer string) {
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
	rpath := config.NormalizePath(r.URL.Path[len(apiDirPath):])
	destination := filepath.Join(cDir, *rpath)
	if !strings.HasPrefix(filepath.Clean(destination)+config.SeparatorString, cDir) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ApiErrorResponse{
			Error: new(string("bad request")),
		})
		log.Printf("User tried accessing %s via API but was denied.", destination)
		return
	}
	if childDirs, childFiles, ch, err := s.getChildren(destination, *rpath); err == nil {
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

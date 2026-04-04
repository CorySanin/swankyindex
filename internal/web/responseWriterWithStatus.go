package web

import "net/http"

type responseWriterWithStatus struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	writeErr    error
}

func (rw *responseWriterWithStatus) WriteHeader(status int) {
	if rw.wroteHeader {
		return
	}
	rw.status = status
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriterWithStatus) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	n, err := rw.ResponseWriter.Write(b)
	if err != nil {
		rw.writeErr = err
	}
	return n, err
}

func (rw *responseWriterWithStatus) success() bool {
	if !rw.wroteHeader {
		return false
	}
	if rw.status >= 400 {
		return false
	}
	return rw.writeErr == nil
}

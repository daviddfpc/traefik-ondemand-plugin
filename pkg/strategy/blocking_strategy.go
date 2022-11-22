package strategy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type CustomResponseWriter struct {
	body       []byte
	statusCode int
	header     http.Header
}

func NewCustomResponseWriter() *CustomResponseWriter {
	return &CustomResponseWriter{
		header: http.Header{},
	}
}

func (w *CustomResponseWriter) Header() http.Header {
	return w.header
}

func (w *CustomResponseWriter) StatusCode() int {
	return w.statusCode
}

func (w *CustomResponseWriter) Write(b []byte) (int, error) {
	w.body = b
	// implement it as per your requirement 
	return 0, nil
}

func (w *CustomResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *CustomResponseWriter) PassResponse(rw http.ResponseWriter) (int, error) {
	for (key, values) := range w.header) {
		rw.Header()[key] := values
	}
	rw.WriteHeader(w.statusCode)
	return rw.Write(w.body)
}

type BlockingStrategy struct {
	Requests           []string
	Name               string
	Next               http.Handler
	Timeout            time.Duration
	BlockDelay         time.Duration
	BlockCheckInterval time.Duration
}

type InternalServerError struct {
	ServiceName string `json:"serviceName"`
	Error       string `json:"error"`
}

// ServeHTTP retrieve the service status
func (e *BlockingStrategy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	for start := time.Now(); time.Since(start) < e.BlockDelay; {
		notReadyCount := 0
		for _, request := range e.Requests {

			log.Printf("Sending request: %s", request)
			status, err := getServiceStatus(request)
			log.Printf("Status: %s", status)

			if err != nil {
				rw.Header().Set("Content-Type", "application/json")
				rw.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(rw).Encode(InternalServerError{ServiceName: e.Name, Error: err.Error()})
				return
			}

			if status != "started" {
				notReadyCount++
			}
		}
		if notReadyCount == 0 {
			// Services all started forward request
			w := NewCustomResponseWriter()
			e.Next.ServeHTTP(w, req)
			while (w.StatusCode() == 502 && time.Since(start) < e.BlockDelay) {
				time.Sleep(e.BlockCheckInterval)
				w := NewCustomResponseWriter()
				e.Next.ServeHTTP(w, req)
			}
			e.PassResponse(rw)
			return
		}

		time.Sleep(e.BlockCheckInterval)
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(rw).Encode(InternalServerError{ServiceName: e.Name, Error: fmt.Sprintf("Service was unreachable within %s", e.BlockDelay)})
}

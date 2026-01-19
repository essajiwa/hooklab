package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"testing"
)

// errorReader is a custom reader that always returns an error
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	m.Run()
}

// errorResponseWriter is a custom http.ResponseWriter that simulates an error on Write
type errorResponseWriter struct {
	header http.Header
	status int
}

func (erw *errorResponseWriter) Header() http.Header {
	if erw.header == nil {
		erw.header = make(http.Header)
	}
	return erw.header
}

func (erw *errorResponseWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write error")
}

func (erw *errorResponseWriter) WriteHeader(statusCode int) {
	erw.status = statusCode
}

// noFlushWriter simulates a ResponseWriter without http.Flusher support.
type noFlushWriter struct {
	header http.Header
	status int
}

func (nfw *noFlushWriter) Header() http.Header {
	if nfw.header == nil {
		nfw.header = make(http.Header)
	}
	return nfw.header
}

func (nfw *noFlushWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nfw *noFlushWriter) WriteHeader(statusCode int) {
	nfw.status = statusCode
}

type sseWriter struct {
	header http.Header
	buffer bytes.Buffer
}

func (sw *sseWriter) Header() http.Header {
	if sw.header == nil {
		sw.header = make(http.Header)
	}
	return sw.header
}

func (sw *sseWriter) Write(p []byte) (int, error) {
	return sw.buffer.Write(p)
}

func (sw *sseWriter) WriteHeader(statusCode int) {
	// no-op for tests
}

func (sw *sseWriter) Flush() {}

package middleware

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

const (
	compressionLib string = "gzip" // compression algorythm
)

type MiddlewareGzip struct {
	logger *zap.Logger
}

func NewMiddlewareGzip(zl *zap.Logger) *MiddlewareGzip {
	return &MiddlewareGzip{
		logger: zl,
	}
}

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	size, err := w.Writer.Write(b)
	if err != nil {
		return 0, fmt.Errorf("error in writing with gzip writer: %w", err)
	}
	return size, nil
}

func (l *MiddlewareGzip) GzipHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := l.logger
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, compressionLib)
		if sendsGzip {
			gr, err := gzip.NewReader(r.Body)
			defer func() {
				if err := gr.Close(); err != nil {
					logger.Sugar().Error(zap.Error(err))
					http.Error(w, "", http.StatusInternalServerError)
				}
			}()
			if err != nil {
				logger.Sugar().Error(zap.Error(err))
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			r.Body = gr
		}

		supportsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), compressionLib)
		if supportsGzip {
			gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
			if err != nil {
				logger.Sugar().Error("error creating gzip writer.")
			}
			w.Header().Set("Content-Encoding", compressionLib)
			defer func() {
				if err := gz.Close(); err != nil {
					logger.Sugar().Error(zap.Error(err))
					http.Error(w, "", http.StatusInternalServerError)
				}
			}()
			h.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

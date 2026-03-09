package middleware

import (
	"net/http"
	"strings"
)

type TransformConfig struct {
	Request  RequestTransform
	Response ResponseTransform
}

type RequestTransform struct {
	AddHeaders    map[string]string
	RemoveHeaders []string
	StripPrefix   string
}

type ResponseTransform struct {
	AddHeaders    map[string]string
	RemoveHeaders []string
}

func Transform(cfg TransformConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, val := range cfg.Request.AddHeaders {
				r.Header.Set(key, val)
			}

			for _, key := range cfg.Request.RemoveHeaders {
				r.Header.Del(key)
			}

			if cfg.Request.StripPrefix != "" {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, cfg.Request.StripPrefix)
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}

			if len(cfg.Response.AddHeaders) > 0 || len(cfg.Response.RemoveHeaders) > 0 {
				wrapped := &transformWriter{
					ResponseWriter: w,
					addHeaders:     cfg.Response.AddHeaders,
					removeHeaders:  cfg.Response.RemoveHeaders,
					headerWritten:  false,
				}
				next.ServeHTTP(wrapped, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type transformWriter struct {
	http.ResponseWriter
	addHeaders    map[string]string
	removeHeaders []string
	headerWritten bool
}

func (tw *transformWriter) WriteHeader(code int) {
	if !tw.headerWritten {
		for key, val := range tw.addHeaders {
			tw.ResponseWriter.Header().Set(key, val)
		}
		for _, key := range tw.removeHeaders {
			tw.ResponseWriter.Header().Del(key)
		}
		tw.headerWritten = true
	}
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *transformWriter) Write(b []byte) (int, error) {
	if !tw.headerWritten {
		tw.WriteHeader(http.StatusOK)
	}
	return tw.ResponseWriter.Write(b)
}

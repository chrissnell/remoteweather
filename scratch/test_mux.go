package main

import (
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	// Register specific routes first
	router.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "NEW handler\n")
	})

	router.HandleFunc("/portal", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "PORTAL handler\n")
	})

	// Register root route
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ROOT handler\n")
	})

	// Test the routes
	testPaths := []string{"/", "/new", "/portal", "/other"}

	for _, path := range testPaths {
		req, _ := http.NewRequest("GET", path, nil)
		var match mux.RouteMatch
		if router.Match(req, &match) {
			fmt.Printf("Path %s: MATCHED\n", path)
			// Get handler name
			w := &testResponseWriter{}
			match.Handler.ServeHTTP(w, req)
			fmt.Printf("  Response: %s", w.body)
		} else {
			fmt.Printf("Path %s: NO MATCH (404)\n", path)
		}
	}
}

type testResponseWriter struct {
	body string
	headers http.Header
	status int
}

func (w *testResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *testResponseWriter) Write(b []byte) (int, error) {
	w.body += string(b)
	return len(b), nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

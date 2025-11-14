package main

import (
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	// Register routes
	router.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "NEW")
	})

	router.HandleFunc("/portal", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "PORTAL")
	})

	// Root with MatcherFunc
	router.MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
		return r.URL.Path == "/"
	}).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ROOT")
	})

	// Walk routes to see what's registered
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		path, _ := route.GetPathTemplate()
		methods, _ := route.GetMethods()
		fmt.Printf("Route: path=%s methods=%v\n", path, methods)
		return nil
	})

	// Test the routes
	fmt.Println("\nTesting routes:")
	testPaths := []string{"/", "/new", "/portal"}

	for _, path := range testPaths {
		req, _ := http.NewRequest("GET", path, nil)
		var match mux.RouteMatch
		if router.Match(req, &match) {
			fmt.Printf("%s: MATCHED\n", path)
		} else {
			fmt.Printf("%s: NO MATCH\n", path)
		}
	}
}

package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type route struct {
	prefix string
	proxy  *httputil.ReverseProxy
}

type Router struct {
	routes []route
}

func NewRouter(prefixToUpstream map[string]string) (*Router, error) {
	r := &Router{}

	for prefix, upstream := range prefixToUpstream {
		target, err := url.Parse(upstream)
		if err != nil {
			return nil, err
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		r.routes = append(r.routes, route{
			prefix: prefix,
			proxy:  proxy,
		})
	}

	return r, nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var best *route

	for i := range r.routes {
		rt := &r.routes[i]

		if !matchesPrefix(req.URL.Path, rt.prefix) {
			continue
		}

		if best == nil || len(rt.prefix) > len(best.prefix) {
			best = rt
		}
	}

	if best == nil {
		http.NotFound(w, req)
		return
	}

	log.Printf("route=%s path=%s", best.prefix, req.URL.Path)

	best.proxy.ServeHTTP(w, req)
}

func matchesPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}

	return strings.HasPrefix(path, prefix+"/")
}

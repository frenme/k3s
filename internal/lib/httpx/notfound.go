package httpx

import "net/http"

func JSONErrors(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		probe := &statusProbe{header: http.Header{}}
		h.ServeHTTP(probe, r)
		if allow := probe.header.Get("Allow"); allow != "" {
			w.Header().Set("Allow", allow)
		}
		WriteError(w, standard(probe.status))
	})
}

type statusProbe struct {
	header http.Header
	status int
}

func (p *statusProbe) Header() http.Header { return p.header }

func (p *statusProbe) WriteHeader(status int) {
	if p.status == 0 {
		p.status = status
	}
}

func (p *statusProbe) Write(b []byte) (int, error) {
	p.WriteHeader(http.StatusOK)
	return len(b), nil
}

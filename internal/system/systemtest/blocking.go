package systemtest

import (
	"io"
	"net/http"
	"net/http/httptest"
)

func newBlockingServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
}

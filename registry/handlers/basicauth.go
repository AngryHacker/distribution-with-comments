// +build go1.4

package handlers

import (
	"net/http"
)

// go1.4+ 中的 basicAuth
func basicAuth(r *http.Request) (username, password string, ok bool) {
	return r.BasicAuth()
}

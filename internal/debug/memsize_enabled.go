//go:build !go1.20
// +build !go1.20

package debug

import (
	"net/http"

	"github.com/fjl/memsize/memsizeui"
)

var Memsize memsizeui.Handler

func registerMemsizeHandler() {
	http.Handle("/memsize/", http.StripPrefix("/memsize", &Memsize))
}

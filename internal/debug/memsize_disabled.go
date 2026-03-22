//go:build go1.20
// +build go1.20

package debug

type memsizeHandler struct{}

func (memsizeHandler) Add(string, interface{}) {}

var Memsize memsizeHandler

func registerMemsizeHandler() {}

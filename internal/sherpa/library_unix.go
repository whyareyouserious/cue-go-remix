//go:build darwin || linux

package sherpa

import (
	"path/filepath"
	"runtime"

	"github.com/ebitengine/purego"
)

var libraryName = func() string {
	if runtime.GOOS == "darwin" {
		return "libsherpa-onnx-c-api.dylib"
	}
	return "libsherpa-onnx-c-api.so"
}()

func openLibrary(dir string) (uintptr, error) {
	return purego.Dlopen(filepath.Join(dir, libraryName), purego.RTLD_NOW|purego.RTLD_LOCAL)
}
func lookupSymbol(h uintptr, name string) (uintptr, error) { return purego.Dlsym(h, name) }

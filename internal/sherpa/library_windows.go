package sherpa

import (
	"golang.org/x/sys/windows"
	"path/filepath"
)

const libraryName = "sherpa-onnx-c-api.dll"

func openLibrary(dir string) (uintptr, error) {
	h, err := windows.LoadLibraryEx(filepath.Join(dir, libraryName), 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	return uintptr(h), err
}
func lookupSymbol(h uintptr, name string) (uintptr, error) {
	return windows.GetProcAddress(windows.Handle(h), name)
}

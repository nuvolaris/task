package templater

import (
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig"
	"github.com/nuvolaris/sh/v3/shell"
	"github.com/nuvolaris/sh/v3/syntax"
)

var templateFuncs template.FuncMap

func init() {
	taskFuncs := template.FuncMap{
		"OS":   func() string { return runtime.GOOS },
		"ARCH": func() string { return runtime.GOARCH },
		"catLines": func(s string) string {
			s = strings.ReplaceAll(s, "\r\n", " ")
			return strings.ReplaceAll(s, "\n", " ")
		},
		"splitLines": func(s string) []string {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			return strings.Split(s, "\n")
		},
		"fromSlash": func(path string) string {
			return filepath.FromSlash(path)
		},
		"toSlash": func(path string) string {
			return filepath.ToSlash(path)
		},
		"exeExt": func() string {
			if runtime.GOOS == "windows" {
				return ".exe"
			}
			return ""
		},
		"shellQuote": func(str string) (string, error) {
			return syntax.Quote(str, syntax.LangBash)
		},
		"splitArgs": func(s string) ([]string, error) {
			return shell.Fields(s, nil)
		},
		// IsSH is deprecated.
		"IsSH": func() bool { return true },
		"joinPath": func(elem ...string) string {
			return filepath.Join(elem...)
		},
		"relPath": func(basePath, targetPath string) (string, error) {
			return filepath.Rel(basePath, targetPath)
		},
	}
	// Deprecated aliases for renamed functions.
	taskFuncs["FromSlash"] = taskFuncs["fromSlash"]
	taskFuncs["ToSlash"] = taskFuncs["toSlash"]
	taskFuncs["ExeExt"] = taskFuncs["exeExt"]

	templateFuncs = sprig.TxtFuncMap()
	for k, v := range taskFuncs {
		templateFuncs[k] = v
	}
}

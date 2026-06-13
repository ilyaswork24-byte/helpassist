package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"helpassist/internal/ollama"
	"helpassist/internal/ui"
)

// skipDir lists directory names never descended into during search.
var skipDir = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".cache": true,
}

// ---- glob ----

type globTool struct{}

func (globTool) Name() string { return "glob" }

func (globTool) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "glob",
			Description: "Найти файлы по шаблону имени, например '*.go' или 'cmd/*.go'. Поиск рекурсивный по подкаталогам.",
			Parameters: schema(map[string]any{
				"pattern": strProp("Шаблон имени файла (синтаксис filepath.Match), напр. *.go"),
				"dir":     strProp("Каталог для поиска. По умолчанию текущий."),
			}, "pattern"),
		},
	}
}

func (globTool) Run(_ context.Context, args map[string]any) (string, error) {
	pattern := argString(args, "pattern")
	dir := argString(args, "dir")
	if dir == "" {
		dir = "."
	}
	if pattern == "" {
		return "", fmt.Errorf("не указан pattern")
	}
	ui.Tool("glob", pattern)

	var matches []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if ok, _ := filepath.Match(pattern, d.Name()); ok {
			matches = append(matches, path)
		}
		return nil
	})

	ui.ToolResult(fmt.Sprintf("%d совпадений", len(matches)))
	if len(matches) == 0 {
		return "Совпадений не найдено.", nil
	}
	return strings.Join(matches, "\n"), nil
}

// ---- grep ----

type grepTool struct{}

func (grepTool) Name() string { return "grep" }

func (grepTool) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "grep",
			Description: "Искать регулярное выражение по содержимому файлов рекурсивно. Возвращает совпадающие строки с путём и номером строки.",
			Parameters: schema(map[string]any{
				"pattern": strProp("Регулярное выражение (синтаксис Go regexp)."),
				"dir":     strProp("Каталог для поиска. По умолчанию текущий."),
				"glob":    strProp("Необязательный фильтр имени файла, напр. *.go"),
			}, "pattern"),
		},
	}
}

func (grepTool) Run(ctx context.Context, args map[string]any) (string, error) {
	pattern := argString(args, "pattern")
	dir := argString(args, "dir")
	fileGlob := argString(args, "glob")
	if dir == "" {
		dir = "."
	}
	if pattern == "" {
		return "", fmt.Errorf("не указан pattern")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("неверное регулярное выражение: %w", err)
	}
	ui.Tool("grep", pattern)

	var out []string
	const maxHits = 200
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if skipDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if fileGlob != "" {
			if ok, _ := filepath.Match(fileGlob, d.Name()); !ok {
				return nil
			}
		}
		if len(out) >= maxHits {
			return filepath.SkipAll
		}
		data, err := os.ReadFile(path)
		if err != nil || isBinary(data) {
			return nil
		}
		for i, ln := range strings.Split(string(data), "\n") {
			if re.MatchString(ln) {
				out = append(out, fmt.Sprintf("%s:%d:%s", path, i+1, strings.TrimSpace(ln)))
				if len(out) >= maxHits {
					break
				}
			}
		}
		return nil
	})

	ui.ToolResult(fmt.Sprintf("%d совпадений", len(out)))
	if len(out) == 0 {
		return "Совпадений не найдено.", nil
	}
	return strings.Join(out, "\n"), nil
}

// isBinary heuristically detects binary content by looking for NUL bytes.
func isBinary(data []byte) bool {
	n := len(data)
	if n > 1024 {
		n = 1024
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helpassist/internal/ollama"
	"helpassist/internal/ui"
)

const maxReadBytes = 200 * 1024 // safety cap for reads

// ---- read_file ----

type readFile struct{}

func (readFile) Name() string { return "read_file" }

func (readFile) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "read_file",
			Description: "Прочитать текстовый файл и вернуть его содержимое с номерами строк.",
			Parameters: schema(map[string]any{
				"path": strProp("Путь к файлу (относительный или абсолютный)."),
			}, "path"),
		},
	}
}

func (readFile) Run(_ context.Context, args map[string]any) (string, error) {
	path := argString(args, "path")
	if path == "" {
		return "", fmt.Errorf("не указан path")
	}
	ui.Tool("read_file", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	truncated := false
	if len(data) > maxReadBytes {
		data = data[:maxReadBytes]
		truncated = true
	}
	lines := strings.Split(string(data), "\n")
	var b strings.Builder
	for i, ln := range lines {
		fmt.Fprintf(&b, "%6d\t%s\n", i+1, ln)
	}
	if truncated {
		fmt.Fprintf(&b, "[файл обрезан до %d КБ — показаны не все строки]\n", maxReadBytes/1024)
	}
	ui.ToolResult(fmt.Sprintf("%d строк", len(lines)))
	return b.String(), nil
}

// ---- list_dir ----

type listDir struct{}

func (listDir) Name() string { return "list_dir" }

func (listDir) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "list_dir",
			Description: "Список файлов и подкаталогов в каталоге.",
			Parameters: schema(map[string]any{
				"path": strProp("Путь к каталогу. По умолчанию текущий каталог."),
			}),
		},
	}
}

func (listDir) Run(_ context.Context, args map[string]any) (string, error) {
	path := argString(args, "path")
	if path == "" {
		path = "."
	}
	ui.Tool("list_dir", path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name + "\n")
	}
	ui.ToolResult(fmt.Sprintf("%d элементов", len(entries)))
	return b.String(), nil
}

// ---- write_file ----

type writeFile struct{ confirm *Confirmer }

func (writeFile) Name() string { return "write_file" }

func (writeFile) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "write_file",
			Description: "Создать новый файл или полностью перезаписать существующий указанным содержимым.",
			Parameters: schema(map[string]any{
				"path":    strProp("Путь к файлу для записи."),
				"content": strProp("Полное новое содержимое файла."),
			}, "path", "content"),
		},
	}
}

func (w writeFile) Run(_ context.Context, args map[string]any) (string, error) {
	path := argString(args, "path")
	content := argString(args, "content")
	if path == "" {
		return "", fmt.Errorf("не указан path")
	}

	preview := content
	if len(preview) > 600 {
		preview = preview[:600] + "\n…(обрезано)"
	}
	verb := "Создать файл"
	if _, err := os.Stat(path); err == nil {
		verb = "Перезаписать файл"
	}
	if !w.confirm.Ask(fmt.Sprintf("%s %s (%d байт):", verb, path, len(content)), preview) {
		return "Пользователь отклонил запись файла.", nil
	}

	ui.Tool("write_file", path)
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	ui.ToolResult(fmt.Sprintf("записано %d байт", len(content)))
	return fmt.Sprintf("Файл %s записан (%d байт).", path, len(content)), nil
}

// ---- edit_file ----

type editFile struct{ confirm *Confirmer }

func (editFile) Name() string { return "edit_file" }

func (editFile) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "edit_file",
			Description: "Заменить точное вхождение old_string на new_string в файле. old_string должна встречаться ровно один раз.",
			Parameters: schema(map[string]any{
				"path":       strProp("Путь к редактируемому файлу."),
				"old_string": strProp("Точный фрагмент, который нужно заменить."),
				"new_string": strProp("Новый фрагмент."),
			}, "path", "old_string", "new_string"),
		},
	}
}

func (e editFile) Run(_ context.Context, args map[string]any) (string, error) {
	path := argString(args, "path")
	oldStr := argString(args, "old_string")
	newStr := argString(args, "new_string")
	if path == "" || oldStr == "" {
		return "", fmt.Errorf("нужны path и old_string")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	src := string(data)
	n := strings.Count(src, oldStr)
	if n == 0 {
		return "old_string не найдена в файле — правка не применена.", nil
	}
	if n > 1 {
		return fmt.Sprintf("old_string встречается %d раз — уточните фрагмент, чтобы он был уникальным.", n), nil
	}

	diff := "- " + strings.ReplaceAll(oldStr, "\n", "\n- ") + "\n+ " + strings.ReplaceAll(newStr, "\n", "\n+ ")
	if !e.confirm.Ask(fmt.Sprintf("Изменить файл %s:", path), diff) {
		return "Пользователь отклонил правку файла.", nil
	}

	ui.Tool("edit_file", path)
	updated := strings.Replace(src, oldStr, newStr, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", err
	}
	ui.ToolResult("правка применена")
	return fmt.Sprintf("Файл %s изменён.", path), nil
}

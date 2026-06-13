package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"helpassist/internal/ollama"
	"helpassist/internal/ui"
)

// ---- run_bash ----

type runBash struct{ confirm *Confirmer }

func (runBash) Name() string { return "run_bash" }

func (runBash) Schema() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        "run_bash",
			Description: "Выполнить shell-команду в текущем рабочем каталоге и вернуть её stdout/stderr. Используй для сборки, тестов, git и пр.",
			Parameters: schema(map[string]any{
				"command": strProp("Команда bash для выполнения."),
			}, "command"),
		},
	}
}

func (r runBash) Run(ctx context.Context, args map[string]any) (string, error) {
	command := argString(args, "command")
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("не указана command")
	}
	if !r.confirm.Ask("Выполнить команду:", command) {
		return "Пользователь отклонил выполнение команды.", nil
	}

	ui.Tool("run_bash", command)
	// Derive from the turn ctx so Ctrl-C interrupts a running command,
	// while still enforcing a hard 120s cap.
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	out, err := cmd.CombinedOutput()
	result := string(out)

	if ctx.Err() == context.DeadlineExceeded {
		ui.ToolResult("прервано по таймауту (120с)")
		return result + "\n[прервано по таймауту 120с]", nil
	}
	if ctx.Err() == context.Canceled {
		ui.ToolResult("прервано пользователем")
		return result + "\n[выполнение прервано]", nil
	}
	if err != nil {
		ui.ToolResult("код выхода != 0: " + err.Error())
		// Return output + error to the model so it can react, not a Go error.
		return fmt.Sprintf("%s\n[завершилось с ошибкой: %v]", result, err), nil
	}
	ui.ToolResult(strings.TrimSpace(firstLine(result)))
	if strings.TrimSpace(result) == "" {
		return "(команда завершилась успешно, без вывода)", nil
	}
	return result, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if s == "" {
		return "успешно"
	}
	return s
}

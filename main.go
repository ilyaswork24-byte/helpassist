// Command helpassist is a local AI coding assistant CLI powered by qwen3 via Ollama.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"helpassist/internal/agent"
	"helpassist/internal/ollama"
	"helpassist/internal/tools"
	"helpassist/internal/ui"
)

const version = "0.1.0"

func main() {
	var (
		modelFlag = flag.String("model", "", "модель Ollama (по умолчанию qwen3:8b или $HELPASSIST_MODEL)")
		yesFlag   = flag.Bool("yes", false, "авто-подтверждать запись файлов и команды (без запроса)")
		thinkFlag = flag.Bool("think", false, "включить режим рассуждений qwen3")
		showVer   = flag.Bool("version", false, "показать версию и выйти")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("helpassist %s\n", version)
		return
	}

	client := ollama.New()
	if *modelFlag != "" {
		client.Model = *modelFlag
	}

	// Verify Ollama is reachable before starting.
	if err := client.Ping(context.Background()); err != nil {
		ui.Error("%v", err)
		ui.Info("Убедитесь, что Ollama запущен (ollama serve) и модель загружена (ollama pull %s).", client.Model)
		os.Exit(1)
	}

	stdin := bufio.NewReader(os.Stdin)
	confirm := tools.NewConfirmer(stdin, *yesFlag)
	registry := tools.NewRegistry(confirm)
	ag := agent.New(client, registry, *thinkFlag)

	// One-shot mode: any positional args are treated as a single prompt.
	// Here Ctrl-C cancels the turn and exits the program.
	if args := flag.Args(); len(args) > 0 {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		prompt := strings.Join(args, " ")
		if err := ag.Turn(ctx, prompt); err != nil {
			if errors.Is(err, context.Canceled) {
				ui.Info("прервано")
				os.Exit(130)
			}
			ui.Error("%v", err)
			os.Exit(1)
		}
		return
	}

	runREPL(ag, client, stdin)
}

func runREPL(ag *agent.Agent, client *ollama.Client, stdin *bufio.Reader) {
	printBanner(client)

	for {
		fmt.Print(ui.Bold(ui.Blue("helpassist› ")))
		line, err := stdin.ReadString('\n')
		if err != nil { // EOF (Ctrl-D) or read error
			fmt.Println()
			ui.Info("До встречи!")
			return
		}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleCommand(input, ag, client) {
				return // /exit
			}
			continue
		}

		if err := runTurn(ag, input); err != nil {
			if errors.Is(err, context.Canceled) {
				ui.Info("прервано")
			} else {
				ui.Error("%v", err)
			}
		}
		fmt.Println()
	}
}

// runTurn executes one turn with a fresh context bound to SIGINT, so Ctrl-C
// cancels just the in-flight turn (returning to the prompt) rather than killing
// the program. The handler is installed only for the duration of the turn.
func runTurn(ag *agent.Agent, input string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ag.Turn(ctx, input)
}

// handleCommand runs a slash command. Returns true if the REPL should exit.
func handleCommand(input string, ag *agent.Agent, client *ollama.Client) bool {
	fields := strings.Fields(input)
	cmd := fields[0]
	arg := ""
	if len(fields) > 1 {
		arg = strings.Join(fields[1:], " ")
	}

	switch cmd {
	case "/exit", "/quit", "/q":
		ui.Info("До встречи!")
		return true
	case "/help", "/?":
		printHelp()
	case "/reset":
		ag.Reset()
		ui.Info("Контекст очищен.")
	case "/think":
		switch arg {
		case "on":
			ag.SetThink(true)
			ui.Info("Режим рассуждений включён.")
		case "off":
			ag.SetThink(false)
			ui.Info("Режим рассуждений выключен.")
		default:
			ui.Info("Рассуждения сейчас: %v. Использование: /think on|off", ag.Think())
		}
	case "/model":
		if arg == "" {
			ui.Info("Текущая модель: %s", client.Model)
		} else {
			client.Model = arg
			ui.Info("Модель переключена на %s", arg)
		}
	case "/cwd":
		if wd, err := os.Getwd(); err == nil {
			ui.Info("Рабочий каталог: %s", wd)
		}
	default:
		ui.Error("неизвестная команда %q (см. /help)", cmd)
	}
	return false
}

func printBanner(client *ollama.Client) {
	fmt.Println(ui.Bold(ui.Cyan("helpassist")) + ui.Gray(" v"+version) + ui.Gray(" — локальный ИИ-ассистент на "+client.Model))
	fmt.Println(ui.Gray("Рабочий каталог: ") + currentDir())
	fmt.Println(ui.Gray("Введите запрос или /help для списка команд. /exit — выход."))
	fmt.Println()
}

func printHelp() {
	fmt.Println(ui.Bold("Команды:"))
	rows := [][2]string{
		{"/help", "показать эту справку"},
		{"/reset", "очистить контекст беседы"},
		{"/think on|off", "переключить режим рассуждений qwen3"},
		{"/model [имя]", "показать или сменить модель Ollama"},
		{"/cwd", "показать рабочий каталог"},
		{"/exit", "выход (или Ctrl-D)"},
	}
	for _, r := range rows {
		fmt.Printf("  %s  %s\n", ui.Cyan(fmt.Sprintf("%-16s", r[0])), ui.Gray(r[1]))
	}
}

func currentDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "(неизвестно)"
}

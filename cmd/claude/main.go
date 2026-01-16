package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-code-go/internal/agent"
	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/config"
	"github.com/anthropics/claude-code-go/internal/tools"
	"github.com/anthropics/claude-code-go/internal/ui"
)

var (
	version = "0.1.0"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claude [prompt]",
		Short: "Claude Code - AI-powered coding assistant",
		Long: `Claude Code is an AI-powered CLI tool that helps with software engineering tasks.
It can read, write, and edit files, execute commands, search code, and more.`,
		RunE:         runMain,
		SilenceUsage: true,
	}

	rootCmd.Flags().StringP("model", "m", "", "Model to use (default: claude-sonnet-4-20250514)")
	rootCmd.Flags().Bool("version", false, "Show version information")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	// Check for version flag
	if v, _ := cmd.Flags().GetBool("version"); v {
		fmt.Printf("Claude Code v%s\n", version)
		return nil
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override model if specified
	if model, _ := cmd.Flags().GetString("model"); model != "" {
		cfg.Model = model
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create terminal UI
	terminal := ui.NewTerminal()

	// Create API client
	credential, authType := cfg.GetAuthCredential()
	clientOpts := []api.ClientOption{
		api.WithModel(cfg.Model),
		api.WithMaxTokens(cfg.MaxTokens),
	}
	if cfg.BaseURL != "" {
		clientOpts = append(clientOpts, api.WithBaseURL(cfg.BaseURL))
	}
	// Set auth type based on configuration
	if authType == config.AuthTypeBearer {
		clientOpts = append(clientOpts, api.WithAuthType(api.AuthTypeBearer))
	}
	client := api.NewClient(credential, clientOpts...)

	// Create tool registry
	registry := tools.NewRegistry()
	todoList := tools.NewTodoList()

	// Register tools
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewReadTool(workDir))
	registry.Register(tools.NewWriteTool(workDir))
	registry.Register(tools.NewEditTool(workDir))
	registry.Register(tools.NewGlobTool(workDir))
	registry.Register(tools.NewGrepTool(workDir))
	registry.Register(tools.NewWebFetchTool())
	registry.Register(tools.NewTodoWriteTool(todoList))

	// Create ask user question tool with handler
	askTool := tools.NewAskUserQuestionTool(func(questions []tools.Question) (map[string]string, error) {
		answers := make(map[string]string)
		for _, q := range questions {
			fmt.Println()
			fmt.Println(q.Question)
			for i, opt := range q.Options {
				fmt.Printf("  %d. %s - %s\n", i+1, opt.Label, opt.Description)
			}
			fmt.Print("Enter your choice (number or text): ")

			line, err := terminal.ReadLine()
			if err != nil {
				return nil, err
			}
			answers[q.Header] = line
		}
		return answers, nil
	})
	registry.Register(askTool)

	// Create agent
	a := agent.NewAgent(client, registry, workDir)

	// Set up event handler
	a.SetEventHandler(func(event agent.Event) {
		switch event.Type {
		case agent.EventTypeText:
			terminal.PrintAssistantText(event.Text)

		case agent.EventTypeToolUseStart:
			terminal.EndAssistantResponse()
			terminal.PrintToolStart(event.ToolName, event.ToolID)

		case agent.EventTypeToolUseEnd:
			terminal.PrintToolEnd(event.ToolName, event.ToolResult, event.IsError)

		case agent.EventTypeError:
			terminal.PrintError(event.Error)

		case agent.EventTypeConversationEnd:
			terminal.EndAssistantResponse()
		}
	})

	// Handle signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted. Exiting...")
		cancel()
	}()

	// If prompt provided as argument, run non-interactively
	if len(args) > 0 {
		prompt := strings.Join(args, " ")
		return a.Chat(ctx, prompt)
	}

	// Interactive mode
	terminal.PrintWelcome()
	terminal.PrintInfo(fmt.Sprintf("Model: %s", client.GetModel()))
	terminal.PrintInfo(fmt.Sprintf("API: %s", client.GetBaseURL()))
	terminal.PrintInfo(fmt.Sprintf("Working directory: %s", workDir))
	fmt.Println()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		terminal.PrintPrompt()
		input, err := terminal.ReadLine()
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return nil
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			handled, err := handleCommand(input, terminal, a)
			if err != nil {
				terminal.PrintError(err)
			}
			if handled {
				continue
			}
		}

		// Send to agent
		if err := a.Chat(ctx, input); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			terminal.PrintError(err)
		}
	}
}

func handleCommand(input string, terminal *ui.Terminal, a *agent.Agent) (bool, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false, nil
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		terminal.PrintHelp()
		return true, nil

	case "/clear":
		a.GetConversation().Clear()
		terminal.PrintSuccess("Conversation cleared")
		return true, nil

	case "/exit", "/quit":
		fmt.Println("Goodbye!")
		os.Exit(0)
		return true, nil

	case "/model":
		if len(parts) < 2 {
			terminal.PrintInfo("Current model: " + "claude-sonnet-4-20250514") // TODO: get from client
			return true, nil
		}
		terminal.PrintInfo("Model switching requires restart")
		return true, nil

	default:
		return false, fmt.Errorf("unknown command: %s. Type /help for available commands", cmd)
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-code-go/internal/agent"
	"github.com/anthropics/claude-code-go/internal/agentregistry"
	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/config"
	"github.com/anthropics/claude-code-go/internal/logger"
	"github.com/anthropics/claude-code-go/internal/tools"
	"github.com/anthropics/claude-code-go/internal/ui"
)

var (
	version = "0.4.0"
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
	rootCmd.Flags().Bool("enable-logging", false, "Enable detailed logging to /tmp")
	rootCmd.Flags().Bool("pretty-log", false, "Enable pretty-printed JSON logs")
	rootCmd.Flags().Bool("simple", false, "Use simple terminal mode (no TUI)")

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

	// Initialize logging if enabled
	enableLogging, _ := cmd.Flags().GetBool("enable-logging")
	prettyLog, _ := cmd.Flags().GetBool("pretty-log")
	if enableLogging {
		if err := logger.InitLogger("/tmp", prettyLog); err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer func() {
			if log := logger.GetLogger(); log != nil {
				log.Close()
			}
		}()
	}

	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check for simple mode
	simpleMode, _ := cmd.Flags().GetBool("simple")

	// If prompt provided as argument, always use simple mode
	if len(args) > 0 {
		simpleMode = true
	}

	// Create API client
	credential, authType := cfg.GetAuthCredential()
	clientOpts := []api.ClientOption{
		api.WithModel(cfg.Model),
		api.WithMaxTokens(cfg.MaxTokens),
	}
	if cfg.BaseURL != "" {
		clientOpts = append(clientOpts, api.WithBaseURL(cfg.BaseURL))
	}
	if authType == config.AuthTypeBearer {
		clientOpts = append(clientOpts, api.WithAuthType(api.AuthTypeBearer))
	}
	client := api.NewClient(credential, clientOpts...)

	// Create agent registry and register built-in agents
	agentRegistry := agentregistry.NewRegistry()
	if err := agentregistry.RegisterBuiltinAgents(agentRegistry); err != nil {
		return fmt.Errorf("failed to register built-in agents: %w", err)
	}

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

	if simpleMode {
		return runSimpleMode(client, registry, agentRegistry, workDir, args)
	}

	return runTUIMode(client, registry, agentRegistry, workDir, cfg.Model)
}

// runTUIMode runs the application in TUI mode
func runTUIMode(client *api.Client, registry *tools.Registry, agentRegistry *agentregistry.Registry, workDir, modelName string) error {
	// Create TUI
	tui := ui.NewSimpleTUI(version, "build", modelName, workDir)

	// Create agent
	a := agent.NewAgent(client, registry, agentRegistry, workDir)

	// Get TUI adapter
	adapter := tui.GetAdapter()

	// Register ask user question tool
	askTool := tools.NewAskUserQuestionTool(func(questions []tools.Question) (map[string]string, error) {
		// In TUI mode, we'll use a simple approach for now
		// TODO: Implement proper TUI-based question dialog
		answers := make(map[string]string)
		for _, q := range questions {
			// Default to first option for now
			if len(q.Options) > 0 {
				answers[q.Header] = q.Options[0].Label
			}
		}
		return answers, nil
	})
	registry.Register(askTool)

	// Register plan mode tools
	planEnterTool := tools.NewPlanEnterTool(workDir, func(toAgent string) error {
		err := a.SwitchAgent(toAgent)
		if err == nil {
			adapter.OnAgentSwitch(toAgent)
		}
		return err
	})
	registry.Register(planEnterTool)

	planExitTool := tools.NewPlanExitTool(workDir, func(toAgent string) error {
		err := a.SwitchAgent(toAgent)
		if err == nil {
			adapter.OnAgentSwitch(toAgent)
		}
		return err
	})
	registry.Register(planExitTool)

	// Create task executor
	taskExecutor := &simpleTaskExecutor{
		client:        client,
		agentRegistry: agentRegistry,
		toolRegistry:  registry,
		workDir:       workDir,
	}
	taskTool := tools.NewTaskTool(agentRegistry, taskExecutor)
	registry.Register(taskTool)

	// Set up agent event handler
	a.SetEventHandler(func(event agent.Event) {
		switch event.Type {
		case agent.EventTypeText:
			adapter.OnText(event.Text)

		case agent.EventTypeToolUseStart:
			var inputStr string
			if event.ToolInput != "" {
				inputStr = event.ToolInput
			}
			adapter.OnToolStart(event.ToolName, event.ToolID, inputStr)

		case agent.EventTypeToolUseEnd:
			adapter.OnToolEnd(event.ToolName, event.ToolID, event.ToolResult, event.IsError)

		case agent.EventTypeError:
			adapter.OnError(event.Error)

		case agent.EventTypeConversationEnd:
			adapter.OnDone()

		case agent.EventTypeAgentSwitch:
			adapter.OnAgentSwitch(event.AgentName)

		case agent.EventTypeTokenUsage:
			if event.TokenUsage != nil {
				input, output, cacheRead, cacheWrite := a.GetTokenUsage()
				adapter.OnTokenUpdate(input, output, cacheRead, cacheWrite)
			}

		case agent.EventTypeCompaction:
			adapter.OnCompaction(event.CompactionInfo)
		}
	})

	// Set up message handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tui.SetMessageHandler(func(msg string) error {
		// Handle commands
		if strings.HasPrefix(msg, "/") {
			return handleTUICommand(msg, a, adapter)
		}
		return a.Chat(ctx, msg)
	})

	// Run TUI
	return tui.Run()
}

// handleTUICommand handles commands in TUI mode
func handleTUICommand(input string, a *agent.Agent, adapter *ui.AgentEventAdapter) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		adapter.OnCompaction("Commands: /help, /clear, /exit, /model, /agent, /tokens")
		return nil

	case "/clear":
		a.GetConversation().Clear()
		adapter.OnCompaction("Conversation cleared")
		return nil

	case "/exit", "/quit":
		os.Exit(0)
		return nil

	case "/model":
		adapter.OnCompaction("Current model: " + "claude-sonnet-4-20250514")
		return nil

	case "/agent":
		adapter.OnCompaction("Current agent: " + a.GetCurrentAgent())
		return nil

	case "/tokens":
		input, output, cacheRead, cacheWrite := a.GetTokenUsage()
		adapter.OnCompaction(fmt.Sprintf("Tokens: Input=%d Output=%d Cache=%d Total=%d",
			input, output, cacheRead, input+output+cacheRead+cacheWrite))
		return nil

	default:
		adapter.OnCompaction(fmt.Sprintf("Unknown command: %s. Type /help for available commands", cmd))
		return nil
	}
}

// runSimpleMode runs the application in simple terminal mode
func runSimpleMode(client *api.Client, registry *tools.Registry, agentRegistry *agentregistry.Registry, workDir string, args []string) error {
	// Create terminal UI
	terminal := ui.NewTerminal()

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

	// Create agent with agent registry
	a := agent.NewAgent(client, registry, agentRegistry, workDir)

	// Register plan mode tools with agent switch callback
	planEnterTool := tools.NewPlanEnterTool(workDir, func(toAgent string) error {
		return a.SwitchAgent(toAgent)
	})
	registry.Register(planEnterTool)

	planExitTool := tools.NewPlanExitTool(workDir, func(toAgent string) error {
		return a.SwitchAgent(toAgent)
	})
	registry.Register(planExitTool)

	// Create task executor for subagent execution
	taskExecutor := &simpleTaskExecutor{
		client:        client,
		agentRegistry: agentRegistry,
		toolRegistry:  registry,
		workDir:       workDir,
	}

	// Register task tool (for subagent invocation)
	taskTool := tools.NewTaskTool(agentRegistry, taskExecutor)
	registry.Register(taskTool)

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

		case agent.EventTypeAgentSwitch:
			terminal.EndAssistantResponse()
			terminal.PrintInfo(fmt.Sprintf("Switched to %s agent", event.AgentName))

		case agent.EventTypeTokenUsage:
			if event.TokenUsage != nil {
				input, output, cacheRead, cacheWrite := a.GetTokenUsage()
				terminal.PrintInfo(fmt.Sprintf("Tokens: Input=%d (+%d cache) Output=%d [Total: %d]",
					input, cacheRead, output, input+cacheRead+output+cacheWrite))
			}

		case agent.EventTypeCompaction:
			terminal.EndAssistantResponse()
			terminal.PrintInfo(fmt.Sprintf("Context: %s", event.CompactionInfo))
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
			handled, err := handleSimpleCommand(input, terminal, a)
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

func handleSimpleCommand(input string, terminal *ui.Terminal, a *agent.Agent) (bool, error) {
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
			terminal.PrintInfo("Current model: " + "claude-sonnet-4-20250514")
			return true, nil
		}
		terminal.PrintInfo("Model switching requires restart")
		return true, nil

	case "/agent":
		terminal.PrintInfo("Current agent: " + a.GetCurrentAgent())
		return true, nil

	case "/tokens":
		input, output, cacheRead, cacheWrite := a.GetTokenUsage()
		terminal.PrintInfo(fmt.Sprintf("Tokens: Input=%d Output=%d Cache=%d Total=%d",
			input, output, cacheRead, input+output+cacheRead+cacheWrite))
		return true, nil

	default:
		return false, fmt.Errorf("unknown command: %s. Type /help for available commands", cmd)
	}
}

// simpleTaskExecutor implements tools.TaskExecutor for subagent execution
type simpleTaskExecutor struct {
	client        *api.Client
	agentRegistry *agentregistry.Registry
	toolRegistry  *tools.Registry
	workDir       string
}

func (e *simpleTaskExecutor) ExecuteAgent(ctx context.Context, agentName string, prompt string) (string, error) {
	// Create a new agent instance for the subagent
	subAgent := agent.NewAgent(e.client, e.toolRegistry, e.agentRegistry, e.workDir)

	// Switch to the requested agent
	if err := subAgent.SwitchAgent(agentName); err != nil {
		return "", fmt.Errorf("failed to switch to agent %s: %w", agentName, err)
	}

	// Execute the prompt
	if err := subAgent.Chat(ctx, prompt); err != nil {
		return "", fmt.Errorf("agent execution failed: %w", err)
	}

	// Collect the response from the conversation
	messages := subAgent.GetConversation().GetMessages()
	if len(messages) == 0 {
		return "", fmt.Errorf("no response from agent")
	}

	lastMsg := messages[len(messages)-1]
	if lastMsg.Role != api.RoleAssistant {
		return "", fmt.Errorf("unexpected last message role: %s", lastMsg.Role)
	}

	// Concatenate text content from the last message
	var response strings.Builder
	for _, content := range lastMsg.Content {
		if content.Type == api.ContentTypeText {
			response.WriteString(content.Text)
		}
	}

	return response.String(), nil
}

// formatToolInput formats tool input for display
func formatToolInput(input string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return input
	}
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return input
	}
	return string(formatted)
}

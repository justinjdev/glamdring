package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/internal/tui"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/agents"
	"github.com/justin/glamdring/pkg/auth"
	"github.com/justin/glamdring/pkg/commands"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/index"
	"github.com/justin/glamdring/pkg/mcp"
	"github.com/justin/glamdring/pkg/tools"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

func main() {
	// Handle subcommands before flag parsing.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "login":
			if err := auth.Login(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "logout":
			if err := auth.Logout(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "version", "--version":
			fmt.Printf("glamdring %s\n", version)
			return
		}
	}

	cwd := flag.String("cwd", "", "working directory (defaults to current directory)")
	model := flag.String("model", "", "Claude model to use (overrides settings)")
	yolo := flag.Bool("yolo", false, "auto-approve all tool permissions")
	flag.Parse()

	creds, err := auth.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'glamdring login' to authenticate with your Claude account, or set ANTHROPIC_API_KEY.")
		os.Exit(1)
	}

	workDir := *cwd
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Load settings.
	settings := config.LoadSettings(workDir)
	if *model != "" {
		settings.Model = *model
	}

	// Discover CLAUDE.md files.
	claudeMDProject, claudeMDUser, _ := config.FindClaudeMD(workDir)

	// Discover custom agent definitions.
	agentDefs := agents.NewRegistry(agents.Discover(workDir))

	// Discover slash commands.
	cmdRegistry := commands.NewRegistry(commands.Discover(workDir))

	// Load hooks.
	hookRunner := hooks.NewHookRunner(hooks.LoadHooks(workDir))

	// Create a cancellable context (needed for MCP servers and agent).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Start MCP servers if configured.
	mcpMgr := mcp.NewManager()
	for name, serverCfg := range settings.MCPServers {
		if err := mcpMgr.StartServer(ctx, name, serverCfg); err != nil {
			log.Printf("warning: failed to start MCP server %q: %v", name, err)
		}
	}

	// Build the subagent runner: a closure that wraps agent.Run and bridges
	// the agent.Message channel into the tools.SubagentResult channel.
	subagentRunner := makeSubagentRunner(creds, settings.Model)

	// Open shire index if configured/available.
	var indexDB *index.DB
	if enabled := settings.Indexer.IndexerEnabled(); enabled == nil || *enabled {
		// nil = auto-detect (try to open), true = force on.
		indexDBPath := filepath.Join(workDir, ".shire", "index.db")
		if db, err := index.Open(indexDBPath); err == nil {
			indexDB = db
		} else if enabled != nil && *enabled {
			log.Printf("warning: indexer enabled but index not found at %s: %v", indexDBPath, err)
		}
	}

	// Consolidated cleanup: cancel context first, then close resources.
	defer func() {
		cancel()
		mcpMgr.Close()
		if indexDB != nil {
			indexDB.Close()
		}
	}()

	// Build the tool set including Task, MCP, and index tools.
	// baseTools are the non-MCP tools, stored on the model so we can rebuild
	// the full list when MCP servers change at runtime.
	taskTool := tools.NewTaskTool(subagentRunner, agentDefs, tools.DefaultTools(workDir))
	baseTools := tools.DefaultToolsWithTask(workDir, taskTool)
	if indexDB != nil {
		baseTools = append(baseTools, index.Tools(indexDB)...)
	}
	allTools := make([]tools.Tool, len(baseTools))
	copy(allTools, baseTools)
	allTools = append(allTools, mcpMgr.Tools()...)

	// Build tool descriptions for the system prompt.
	var toolDescs []config.ToolDescription
	for _, t := range allTools {
		toolDescs = append(toolDescs, config.ToolDescription{
			Name:        t.Name(),
			Description: t.Description(),
		})
	}

	envInfo := config.EnvironmentInfo{
		Platform: runtime.GOOS,
		Shell:    os.Getenv("SHELL"),
		CWD:      workDir,
		Date:     time.Now().Format("2006-01-02"),
		Model:    settings.Model,
	}

	systemPrompt := config.BuildSystemPrompt(
		config.DefaultBaseInstructions(),
		toolDescs,
		claudeMDProject,
		claudeMDUser,
		envInfo,
	)

	cfg := agent.Config{
		Model:        settings.Model,
		Creds:        creds,
		SystemPrompt: systemPrompt,
		Tools:        allTools,
		MaxTurns:     settings.MaxTurns,
		CWD:          workDir,
		HookRunner:   hookRunner,
		Yolo:         *yolo,
	}

	m := tui.NewWithAgent(ctx, cfg)
	m.SetSettings(settings)
	m.SetCommandRegistry(cmdRegistry)
	m.SetIndexerConfig(settings.Indexer)
	m.SetMCPManager(mcpMgr)
	m.SetMCPConfiguredCount(len(settings.MCPServers))
	m.SetBaseTools(baseTools)
	if indexDB != nil {
		m.SetIndexDB(indexDB)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	// Wire MCP death callback to send bubbletea message.
	mcpMgr.OnServerDeath = func(name string) {
		p.Send(tui.MCPServerDiedMsg{Name: name})
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Fire SessionEnd hook on clean exit.
	// Use Background context since the signal context may already be cancelled.
	hookRunner.Run(context.Background(), hooks.SessionEnd, "", nil)
}

// makeSubagentRunner returns a SubagentRunner that wraps agent.Run. It
// captures the credentials and model so subagents share the parent's auth.
func makeSubagentRunner(creds auth.Credentials, model string) tools.SubagentRunner {
	return func(ctx context.Context, opts tools.SubagentOptions) <-chan tools.SubagentResult {
		resultCh := make(chan tools.SubagentResult, 64)

		var maxTurns *int
		if opts.MaxTurns > 0 {
			maxTurns = &opts.MaxTurns
		}

		cfg := agent.Config{
			Prompt:       opts.Prompt,
			SystemPrompt: opts.SystemPrompt,
			Creds:        creds,
			Model:        model,
			Tools:        opts.Tools,
			MaxTurns:     maxTurns,
		}

		agentCh := agent.Run(ctx, cfg)

		go func() {
			defer close(resultCh)

			for msg := range agentCh {
				switch msg.Type {
				case agent.MessageTextDelta:
					resultCh <- tools.SubagentResult{Text: msg.Text}

				case agent.MessageToolResult:
					// Include tool results so the parent sees what the
					// subagent discovered, but only the output text.
					// Skip this to keep the result focused on final text.

				case agent.MessageError:
					errText := "unknown error"
					if msg.Err != nil {
						errText = msg.Err.Error()
					}
					resultCh <- tools.SubagentResult{
						Text:    fmt.Sprintf("error: %s", errText),
						IsError: true,
					}

				case agent.MessageDone:
					resultCh <- tools.SubagentResult{Done: true}
					return

				case agent.MessageMaxTurnsReached:
					resultCh <- tools.SubagentResult{
						Text:    "subagent reached maximum turns",
						IsError: true,
						Done:    true,
					}
					return
				}
			}
		}()

		return resultCh
	}
}

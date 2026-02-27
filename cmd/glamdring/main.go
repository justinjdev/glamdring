package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/internal/tui"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/agents"
	"github.com/justin/glamdring/pkg/commands"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/mcp"
	"github.com/justin/glamdring/pkg/tools"
)

func main() {
	cwd := flag.String("cwd", "", "working directory (defaults to current directory)")
	model := flag.String("model", "", "Claude model to use (overrides settings)")
	flag.Parse()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: ANTHROPIC_API_KEY environment variable is not set")
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Start MCP servers if configured.
	mcpMgr := mcp.NewManager()
	defer mcpMgr.Close()
	for name, serverCfg := range settings.MCPServers {
		if err := mcpMgr.StartServer(ctx, name, serverCfg); err != nil {
			log.Printf("warning: failed to start MCP server %q: %v", name, err)
		}
	}

	// Build the subagent runner: a closure that wraps agent.Run and bridges
	// the agent.Message channel into the tools.SubagentResult channel.
	subagentRunner := makeSubagentRunner(apiKey, settings.Model)

	// Build the tool set including Task and MCP tools.
	taskTool := tools.NewTaskTool(subagentRunner, agentDefs, tools.DefaultTools(workDir))
	allTools := tools.DefaultToolsWithTask(workDir, taskTool)
	allTools = append(allTools, mcpMgr.Tools()...)

	// Build tool descriptions for the system prompt.
	var toolDescs []config.ToolDescription
	for _, t := range allTools {
		toolDescs = append(toolDescs, config.ToolDescription{
			Name:        t.Name(),
			Description: t.Description(),
		})
	}

	systemPrompt := config.BuildSystemPrompt(
		config.DefaultBaseInstructions(),
		toolDescs,
		claudeMDProject,
		claudeMDUser,
	)

	cfg := agent.Config{
		Model:        settings.Model,
		APIKey:       apiKey,
		SystemPrompt: systemPrompt,
		Tools:        allTools,
		MaxTurns:     settings.MaxTurns,
		CWD:          workDir,
		HookRunner:   hookRunner,
	}

	m := tui.NewWithAgent(ctx, cfg)
	m.SetCommandRegistry(cmdRegistry)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// makeSubagentRunner returns a SubagentRunner that wraps agent.Run. It
// captures the API key and model so subagents share the parent's credentials.
func makeSubagentRunner(apiKey, model string) tools.SubagentRunner {
	return func(ctx context.Context, opts tools.SubagentOptions) <-chan tools.SubagentResult {
		resultCh := make(chan tools.SubagentResult, 64)

		cfg := agent.Config{
			Prompt:       opts.Prompt,
			SystemPrompt: opts.SystemPrompt,
			APIKey:       apiKey,
			Model:        model,
			Tools:        opts.Tools,
			MaxTurns:     opts.MaxTurns,
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

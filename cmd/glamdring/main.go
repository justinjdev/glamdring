package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/internal/tui"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/config"
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

	// Build tool descriptions for the system prompt.
	allTools := tools.DefaultTools(workDir)
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := agent.Config{
		Model:        settings.Model,
		APIKey:       apiKey,
		SystemPrompt: systemPrompt,
		Tools:        allTools,
		MaxTurns:     settings.MaxTurns,
		CWD:          workDir,
	}

	m := tui.NewWithAgent(ctx, cfg)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

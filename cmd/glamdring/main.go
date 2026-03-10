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
	"github.com/justin/glamdring/pkg/session"
	"github.com/justin/glamdring/pkg/teams"
	"github.com/justin/glamdring/pkg/tools"
	"github.com/justin/glamdring/pkg/update"
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
		case "update":
			runUpdate(version)
			return
		}
	}

	cwd := flag.String("cwd", "", "working directory (defaults to current directory)")
	model := flag.String("model", "", "Claude model to use (overrides settings)")
	yolo := flag.Bool("yolo", false, "auto-approve all tool permissions")
	experimentalTeams := flag.Bool("experimental-teams", false, "enable experimental agent teams support")
	demo := flag.Bool("demo", false, "launch with demo content for theme screenshots")
	demoTheme := flag.String("demo-theme", "glamdring", "theme to use with --demo")
	demoIndexPrompt := flag.Bool("demo-index-prompt", false, "launch in index build prompt state for screenshots")
	flag.Parse()

	// Demo mode: launch TUI with sample content, no auth or agent needed.
	if *demo {
		runDemo(*demoTheme)
		return
	}
	if *demoIndexPrompt {
		runDemoIndexPrompt(*demoTheme)
		return
	}

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

	// Initialize clipboard subsystem (needed for Ctrl+V image paste and /copy).
	if err := tui.InitClipboard(); err != nil {
		log.Printf("warning: clipboard not available: %v", err)
	}

	// Load settings.
	settings := config.LoadSettings(workDir)
	if *model != "" {
		settings.Model = *model
	}

	// Initialize session persistence store.
	var sessionStore *session.Store
	if settings.Persistence.Enabled {
		store, err := session.Open(settings.SessionsDir())
		if err != nil {
			log.Printf("warning: failed to open session store: %v", err)
		} else {
			sessionStore = store
		}
	}

	// Discover instruction files (GLAMDRING.md / CLAUDE.md).
	claudeMDProject, claudeMDUser, _ := config.FindInstructions(workDir)

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
	subagentRunner := makeSubagentRunner(creds, settings.Model, settings.ThinkingBudget)

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
	// baseTools are the non-MCP, non-index tools — stored on the model so
	// refreshMCPTools() can rebuild the full list without duplicating index tools.
	taskTool := tools.NewTaskTool(subagentRunner, agentDefs, tools.DefaultTools(workDir))
	baseTools := tools.DefaultToolsWithTask(workDir, taskTool)

	// Enable team tools if the experimental flag or settings enable teams.
	teamsEnabled := *experimentalTeams || settings.Experimental.Teams
	var teamRegistry *teams.ManagerRegistry
	if teamsEnabled {
		teamRegistry = teams.NewManagerRegistry()
		teamTools := []tools.Tool{
			teams.TeamCreateTool{Registry: teamRegistry},
			teams.TeamDeleteTool{Registry: teamRegistry},
			teams.TaskCreateTool{Registry: teamRegistry},
			teams.TaskListTool{Registry: teamRegistry},
			teams.TaskGetTool{Registry: teamRegistry},
			teams.TaskUpdateTool{Registry: teamRegistry, AgentName: "lead"},
			teams.SendMessageTool{Registry: teamRegistry, AgentName: "lead"},
			teams.AdvancePhaseTool{Registry: teamRegistry, AgentName: "lead"},
		}
		baseTools = append(baseTools, teamTools...)

		// Set up the team setup function on the Task tool.
		taskTool.TeamSetupFunc = makeTeamSetupFunc(teamRegistry, creds, settings)
	}

	allTools := make([]tools.Tool, len(baseTools))
	copy(allTools, baseTools)
	if indexDB != nil {
		allTools = append(allTools, index.Tools(indexDB)...)
	}
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

	// Load permission presets.
	permissions, err := config.LoadPermissions(workDir)
	if err != nil {
		log.Printf("warning: %v (permissions will not be applied)", err)
	}

	cfg := agent.Config{
		Model:          settings.Model,
		Creds:          creds,
		SystemPrompt:   systemPrompt,
		Tools:          allTools,
		MaxTurns:       settings.MaxTurns,
		CWD:            workDir,
		HookRunner:     hookRunner,
		Permissions:    permissions,
		Yolo:           *yolo,
		ThinkingBudget: settings.ThinkingBudget,
		Store:          sessionStore,
	}

	m := tui.NewWithAgent(ctx, cfg)
	m.SetSettings(settings)
	m.SetVersion(version)
	if sessionStore != nil {
		m.SetSessionStore(sessionStore)
		m.ShowSessionRestoreHint()
	}
	m.SetDisableUpdateCheck(settings.DisableUpdateCheck)

	// Apply theme from settings.
	{
		themeName := settings.Theme
		if themeName == "" {
			themeName = "glamdring"
		}
		palette, found := tui.LookupTheme(themeName)
		if settings.Themes != nil {
			if userTheme, ok := settings.Themes[themeName]; ok {
				palette = tui.PaletteFromUserConfig(themeName, userTheme)
				found = true
			}
		}
		if !found && settings.Theme != "" {
			log.Printf("warning: unknown theme %q, falling back to glamdring", settings.Theme)
		}
		if settings.Theme != "" || settings.HighContrast {
			m.SetTheme(palette, settings.HighContrast)
		}
	}

	m.SetCommandRegistry(cmdRegistry)
	m.SetIndexerConfig(settings.Indexer)
	m.SetMCPManager(mcpMgr)
	m.SetMCPConfiguredCount(len(settings.MCPServers))
	m.SetBaseTools(baseTools)
	if indexDB != nil {
		m.SetIndexDB(indexDB)
	}
	m.InitMCPStatus()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Wire MCP death callback to send bubbletea message.
	// Note: there is a small race window between StartServer (which spawns
	// the monitor goroutine) and this assignment. In practice the window is
	// negligible because servers don't die during startup, and a proper fix
	// would require significant restructuring of the init sequence.
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

// runDemo launches the TUI in demo mode with pre-populated content.
// Used for VHS theme screenshot capture.
func runDemo(themeName string) {
	m := tui.New()
	palette, ok := tui.LookupTheme(themeName)
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: unknown theme %q, using glamdring\n", themeName)
	}
	m.SetTheme(palette, false)
	m.PopulateDemoContent()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// runDemoIndexPrompt launches the TUI showing the index build prompt.
// Used for VHS screenshot capture of the startup index prompt feature.
func runDemoIndexPrompt(themeName string) {
	m := tui.New()
	palette, ok := tui.LookupTheme(themeName)
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: unknown theme %q, using glamdring\n", themeName)
	}
	m.SetTheme(palette, false)
	m.PopulateDemoIndexPrompt()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// makeSubagentRunner returns a SubagentRunner that wraps agent.Run. It
// captures the credentials and model so subagents share the parent's auth.
func makeSubagentRunner(creds auth.Credentials, model string, thinkingBudget *int) tools.SubagentRunner {
	return func(ctx context.Context, opts tools.SubagentOptions) <-chan tools.SubagentResult {
		resultCh := make(chan tools.SubagentResult, 64)

		var maxTurns *int
		if opts.MaxTurns > 0 {
			maxTurns = &opts.MaxTurns
		}

		agentModel := model
		if opts.Model != "" {
			agentModel = opts.Model
		}

		cfg := agent.Config{
			Prompt:         opts.Prompt,
			SystemPrompt:   opts.SystemPrompt,
			Creds:          creds,
			Model:          agentModel,
			Tools:          opts.Tools,
			MaxTurns:       maxTurns,
			Yolo:           true, // subagents auto-approve tools
			ThinkingBudget: thinkingBudget,
		}

		// Pass through team state for team agents.
		if opts.TeamState != nil {
			cfg.TeamState = opts.TeamState
			if ts, ok := opts.TeamState.(*teamState); ok {
				cfg.PriorityMessages = ts.priorityCh
				cfg.RegularMessages = ts.regularCh
				cfg.ToolProvider = ts.provider
			}
		}

		agentCh := agent.Run(ctx, cfg)

		go func() {
			defer close(resultCh)

			for msg := range agentCh {
				switch msg.Type {
				case agent.MessageTextDelta:
					resultCh <- tools.SubagentResult{Text: msg.Text}

				case agent.MessageToolResult:
					// Tool results are skipped to keep the parent's output focused on final text.

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

// teamState holds the opaque state passed through agent.Config.TeamState
// for team agents. It carries the message channels and tool provider.
type teamState struct {
	priorityCh <-chan any
	regularCh  <-chan any
	provider   tools.ToolProvider
	mgr        *teams.TeamManager
	agentName  string
}

// phaseConfigToPhases converts settings PhaseConfigs to teams.Phase values.
func phaseConfigToPhases(configs []config.PhaseConfig) []teams.Phase {
	phases := make([]teams.Phase, len(configs))
	for i, c := range configs {
		phases[i] = teams.Phase{
			Name:       c.Name,
			Tools:      c.Tools,
			Model:      c.Model,
			Fallback:   c.Fallback,
			Gate:       teams.GateType(c.Gate),
			GateConfig: c.GateConfig,
		}
	}
	return phases
}

// makeTeamSetupFunc creates the TeamSetupFunc callback that configures
// subagents for team participation. It wires up message channels, phase
// tracking, and team-specific tools.
func makeTeamSetupFunc(registry *teams.ManagerRegistry, creds auth.Credentials, settings config.Settings) tools.TeamSetupFunc {
	return func(ctx context.Context, params tools.TeamSetupParams) (*tools.TeamSetupResult, error) {
		mgr := registry.Get(params.TeamName)
		if mgr == nil {
			return nil, fmt.Errorf("team %q not found", params.TeamName)
		}

		// Register the agent as a team member.
		member := teams.Member{
			Name:   params.AgentName,
			Status: teams.MemberStatusActive,
		}
		if err := mgr.Members.Add(member); err != nil {
			return nil, fmt.Errorf("add member: %w", err)
		}

		// Subscribe to message channels.
		regularCh, priorityCh, err := mgr.Messages.Subscribe(params.AgentName, 32)
		if err != nil {
			return nil, fmt.Errorf("subscribe: %w", err)
		}

		// Wrap channels as chan any for the agent config.
		regularAnyCh := make(chan any, 32)
		priorityAnyCh := make(chan any, 32)
		go func() {
			defer close(regularAnyCh)
			for {
				select {
				case msg, ok := <-regularCh:
					if !ok {
						return
					}
					regularAnyCh <- msg
				case <-ctx.Done():
					return
				}
			}
		}()
		go func() {
			defer close(priorityAnyCh)
			for {
				select {
				case msg, ok := <-priorityCh:
					if !ok {
						return
					}
					priorityAnyCh <- msg
				case <-ctx.Done():
					return
				}
			}
		}()

		// Resolve workflow phases for this agent.
		// Check settings workflows first for user-defined workflows.
		var customPhases []teams.Phase
		if wf, ok := settings.Workflows[params.Workflow]; ok {
			customPhases = phaseConfigToPhases(wf.Phases)
		}
		phases, err := teams.ResolveWorkflow(params.Workflow, customPhases, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve workflow %q: %w", params.Workflow, err)
		}
		if len(phases) > 0 {
			mgr.Phases.SetPhases(params.AgentName, phases)
			if params.StartPhase != "" {
				if _, err := mgr.Phases.AdvanceTo(params.AgentName, params.StartPhase); err != nil {
					return nil, fmt.Errorf("advance to start phase: %w", err)
				}
			}
		}

		// Build the tool registry for this agent. Start with base tools,
		// then add team-specific tools with this agent's name.
		agentRegistry := tools.NewRegistry()
		for _, t := range params.BaseTools {
			agentRegistry.Register(t)
		}

		// Register team tools with this agent's identity.
		agentRegistry.Register(teams.TaskCreateTool{Registry: registry})
		agentRegistry.Register(teams.TaskListTool{Registry: registry})
		agentRegistry.Register(teams.TaskGetTool{Registry: registry})
		agentRegistry.Register(teams.TaskUpdateTool{Registry: registry, AgentName: params.AgentName})
		agentRegistry.Register(teams.SendMessageTool{Registry: registry, AgentName: params.AgentName})

		// agentCancel is wired to AdvancePhaseTool and the cleanup closure so
		// force shutdown and agent teardown both release the child context.
		_, agentCancel := context.WithCancel(ctx)

		agentRegistry.Register(teams.AdvancePhaseTool{
			Registry:   registry,
			AgentName:  params.AgentName,
			PriorityCh: priorityAnyCh,
			CancelFunc: agentCancel,
		})

		// Build the PhaseRegistry as the ToolProvider for this agent.
		var provider tools.ToolProvider
		if len(phases) > 0 {
			provider = teams.NewPhaseRegistry(agentRegistry, mgr.Phases, params.AgentName, nil, nil)
		} else {
			provider = agentRegistry
		}

		// Determine the initial model from the current phase.
		agentModel := settings.Model
		if len(phases) > 0 {
			if phase, _, err := mgr.Phases.Current(params.AgentName); err == nil && phase.Model != "" {
				agentModel = phase.Model
			}
		}

		// Build system prompt with team context.
		var memberNames []string
		for _, m := range mgr.Members.List() {
			memberNames = append(memberNames, m.Name)
		}
		phaseName := ""
		gateType := ""
		if len(phases) > 0 {
			if phase, _, err := mgr.Phases.Current(params.AgentName); err == nil {
				phaseName = phase.Name
				gateType = string(phase.Gate)
			}
		}
		teamPrompt := config.BuildTeamAgentPrompt(config.TeamAgentInfo{
			TeamName:  params.TeamName,
			AgentName: params.AgentName,
			Phase:     phaseName,
			Gate:      gateType,
			Members:   memberNames,
		})

		sysPrompt := params.BaseSysPrompt + teamPrompt

		// Inject context from cache if requested.
		for _, key := range params.InjectContext {
			if val, ok := mgr.Context.Load(key); ok {
				sysPrompt += "\n\n## Injected Context: " + key + "\n\n" + val
			}
		}

		state := &teamState{
			priorityCh: priorityAnyCh,
			regularCh:  regularAnyCh,
			provider:   provider,
			mgr:        mgr,
			agentName:  params.AgentName,
		}

		return &tools.TeamSetupResult{
			Tools:        agentRegistry.All(),
			SystemPrompt: sysPrompt,
			Model:        agentModel,
			TeamState:    state,
			Cleanup: func() {
				agentCancel()
				if err := mgr.CleanupAgent(params.AgentName); err != nil {
					log.Printf("warning: cleanup errors for agent %q: %v", params.AgentName, err)
				}
			},
		}, nil
	}
}

// runUpdate performs a CLI-based update check and install (glamdring update).
func runUpdate(currentVersion string) {
	fmt.Println("Checking for updates...")
	rel, err := update.CheckLatest(currentVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if rel == nil {
		fmt.Printf("glamdring %s is up to date.\n", currentVersion)
		return
	}

	fmt.Printf("Update available: %s -> %s\n", currentVersion, rel.Version)
	fmt.Print("Install? [y/N] ")

	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Update cancelled.")
		return
	}

	fmt.Println("Downloading...")
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := update.Download(rel, exe); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to %s. Restart glamdring to use the new version.\n", rel.Version)
}

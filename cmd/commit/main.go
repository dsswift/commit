// commit is an intelligent git commit tool that creates semantic commits using LLM analysis.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dsswift/commit/internal/analyzer"
	"github.com/dsswift/commit/internal/config"
	"github.com/dsswift/commit/internal/git"
	"github.com/dsswift/commit/internal/interactive"
	"github.com/dsswift/commit/internal/llm"
	"github.com/dsswift/commit/internal/logging"
	"github.com/dsswift/commit/internal/planner"
	"github.com/dsswift/commit/internal/updater"
	"github.com/dsswift/commit/pkg/types"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	// Parse flags
	flags := parseFlags()

	// Handle special flags
	if flags.version {
		fmt.Printf("commit version %s\n", Version)
		// Always check for updates (bypass cache)
		versionInfo := updater.CheckVersionFresh(Version)
		if notice := updater.FormatUpdateNotice(versionInfo); notice != "" {
			fmt.Print(notice)
		}
		return 0
	}

	if flags.upgrade {
		fmt.Println("🔄 Checking for updates...")
		result := updater.Upgrade(Version)
		fmt.Println(updater.FormatUpgradeResult(result))
		if result.Success {
			return 0
		}
		return 1
	}

	// Handle --set flag
	if flags.setConfig != "" {
		return handleSetConfig(flags.setConfig)
	}

	// Handle --diff flag
	if flags.diffFile != "" {
		return handleDiff(flags)
	}

	// Handle --interactive flag
	if flags.interactive {
		return handleInteractive(flags)
	}

	// Generate execution ID and start logging
	executionID := logging.GenerateExecutionID()
	logger, err := logging.NewExecutionLogger(executionID)
	if err != nil {
		// Non-fatal - continue without logging
		logger = nil
	}
	defer func() {
		if logger != nil {
			logger.Close()
		}
	}()

	// Log start
	if logger != nil {
		logger.LogStart(Version, os.Args[1:])
	}

	// Run cleanup in background
	go logging.CleanupOldLogs()

	// Start version check in background
	versionChan := make(chan *updater.VersionInfo, 1)
	go func() {
		versionChan <- updater.CheckVersion(Version)
	}()

	// Execute main logic
	result := execute(flags, logger)

	// Write registry entry
	cwd, _ := os.Getwd()
	gitRoot, _ := git.FindGitRoot(cwd)

	entry := logging.RegistryEntry{
		ExecutionID:    executionID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Version:        Version,
		CWD:            cwd,
		Args:           os.Args[1:],
		GitRoot:        gitRoot,
		DurationMS:     result.Duration.Milliseconds(),
		ExitCode:       result.ExitCode,
		CommitsCreated: len(result.CommitsCreated),
	}
	logging.WriteRegistryEntry(entry)

	// Log completion
	if logger != nil {
		logger.LogComplete(result.ExitCode, len(result.CommitsCreated))
	}

	// Check for version update (non-blocking)
	select {
	case versionInfo := <-versionChan:
		if notice := updater.FormatUpdateNotice(versionInfo); notice != "" {
			fmt.Print(notice)
		}
	default:
		// Version check not complete, don't wait
	}

	return result.ExitCode
}

type flags struct {
	staged      bool
	dryRun      bool
	verbose     bool
	reverse     bool
	force       bool
	interactive bool
	version     bool
	upgrade     bool
	single      bool
	smart       bool
	diffFile    string
	diffFrom    string
	diffTo      string
	provider    string
	setConfig   string
}

func parseFlags() flags {
	f := flags{}

	flag.BoolVar(&f.staged, "staged", false, "Only commit staged files")
	flag.BoolVar(&f.dryRun, "dry-run", false, "Preview commits without creating them")
	flag.BoolVar(&f.verbose, "v", false, "Verbose output")
	flag.BoolVar(&f.verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&f.reverse, "reverse", false, "Reverse HEAD commit into uncommitted changes")
	flag.BoolVar(&f.force, "force", false, "Force operation (for --reverse/--interactive on pushed commits)")
	flag.BoolVar(&f.interactive, "i", false, "Interactive rebase wizard")
	flag.BoolVar(&f.interactive, "interactive", false, "Interactive rebase wizard")
	flag.BoolVar(&f.version, "version", false, "Print version")
	flag.BoolVar(&f.upgrade, "upgrade", false, "Upgrade to latest version")
	flag.StringVar(&f.diffFile, "diff", "", "Analyze changes to a specific file")
	flag.StringVar(&f.diffFrom, "from", "", "Start ref for diff analysis")
	flag.StringVar(&f.diffTo, "to", "", "End ref for diff analysis")
	flag.StringVar(&f.provider, "provider", "", "Override LLM provider")
	flag.BoolVar(&f.single, "single", false, "Create a single commit for all files")
	flag.BoolVar(&f.single, "1", false, "Create a single commit for all files (shorthand)")
	flag.BoolVar(&f.smart, "smart", false, "Create semantic commits (default)")
	flag.StringVar(&f.setConfig, "set", "", "Set config value (e.g., defaultMode=single)")

	flag.Parse()

	return f
}

type executeResult struct {
	ExitCode       int
	Duration       time.Duration
	CommitsCreated []types.ExecutedCommit
}

func execute(flags flags, logger *logging.ExecutionLogger) executeResult {
	startTime := time.Now()
	result := executeResult{}

	// Find git root
	cwd, err := os.Getwd()
	if err != nil {
		printError("Failed to get current directory", err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		printError("Not a git repository", err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	// Handle --reverse
	if flags.reverse {
		result.ExitCode = handleReverse(gitRoot, flags.force, flags.verbose)
		result.Duration = time.Since(startTime)
		return result
	}

	// Load config
	printStep("🔧", "Loading config...")

	userConfig, err := config.LoadUserConfig()
	if err != nil {
		handleConfigError(err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	// Override provider if specified
	if flags.provider != "" {
		userConfig.Provider = flags.provider
	}

	// Override dry-run if configured
	if userConfig.DryRun {
		flags.dryRun = true
	}

	repoConfig, err := config.LoadRepoConfig(gitRoot)
	if err != nil {
		printError("Failed to load repo config", err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	// Log config loaded
	if logger != nil {
		var scopes []string
		for _, s := range repoConfig.Scopes {
			scopes = append(scopes, s.Scope)
		}
		logger.LogConfigLoaded(userConfig.Provider, len(repoConfig.Scopes) > 0, scopes)
	}

	printSuccess(fmt.Sprintf("Provider: %s", userConfig.Provider))
	if len(repoConfig.Scopes) > 0 {
		seen := make(map[string]bool)
		var scopeNames []string
		for _, s := range repoConfig.Scopes {
			if !seen[s.Scope] {
				seen[s.Scope] = true
				scopeNames = append(scopeNames, s.Scope)
			}
		}
		printSuccess(fmt.Sprintf("Scopes (from .commit.json): %s", strings.Join(scopeNames, ", ")))
	}

	// Collect git changes
	printStep("📂", "Collecting changes...")

	collector := git.NewCollector(gitRoot)
	status, err := collector.Status()
	if err != nil {
		printError("Failed to get git status", err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	// Check if there are changes
	var files []string
	if flags.staged {
		files = status.Staged
		if len(files) == 0 {
			printStepError("No staged files")
			printFinal("❌", "Nothing staged to commit")
			fmt.Println("   Stage files with 'git add' first, or run without --staged")
			result.ExitCode = 1
			result.Duration = time.Since(startTime)
			return result
		}
	} else {
		files = status.AllFiles()
		if len(files) == 0 {
			printStepError("No changes found")
			printFinal("❌", "Nothing to commit")
			fmt.Println("   All tracked files are up to date.")
			result.ExitCode = 0
			result.Duration = time.Since(startTime)
			return result
		}
	}

	modified := len(status.Modified)
	added := len(status.Added) + len(status.Untracked)
	printSuccess(fmt.Sprintf("Found %d files (%d modified, %d new)", len(files), modified, added))

	if flags.verbose {
		for _, f := range files {
			scope := config.ResolveScope(f, repoConfig)
			if scope == "" {
				scope = "(no scope)"
			}
			printVerbose(fmt.Sprintf("  %s → %s", f, scope))
		}
	}

	// Build analysis context
	contextBuilder := analyzer.NewContextBuilder(gitRoot, repoConfig)
	analysisReq, err := contextBuilder.Build(flags.staged)
	if err != nil {
		if _, ok := err.(*analyzer.NoChangesError); ok {
			printFinal("❌", "Nothing to commit")
			result.ExitCode = 0
			result.Duration = time.Since(startTime)
			return result
		}
		printError("Failed to build context", err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	// Resolve commit mode: flags override config
	singleMode := flags.single
	if !flags.single && !flags.smart {
		// No explicit flag - use config default
		if userConfig.DefaultMode == "single" {
			singleMode = true
		}
	}
	// --smart flag explicitly overrides config to multi-commit mode
	if flags.smart {
		singleMode = false
	}
	analysisReq.SingleCommit = singleMode

	// Log context built
	if logger != nil {
		var scopes []string
		scopeSet := make(map[string]bool)
		for _, f := range analysisReq.Files {
			if f.Scope != "" && !scopeSet[f.Scope] {
				scopes = append(scopes, f.Scope)
				scopeSet[f.Scope] = true
			}
		}
		logger.LogContextBuilt(len(analysisReq.Files), len(analysisReq.Diff), scopes)
	}

	// Create LLM provider
	printStep("🤖", "Analyzing changes...")

	provider, err := llm.NewProvider(userConfig)
	if err != nil {
		printError("Failed to create LLM provider", err)
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	printProgress(fmt.Sprintf("Sending to %s...", provider.Model()))

	// Log LLM request
	if logger != nil {
		systemPrompt, userPrompt := llm.BuildPrompt(analysisReq)
		logger.LogLLMRequest(provider.Name(), provider.Model(), len(systemPrompt)+len(userPrompt))
	}

	// Call LLM
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	plan, err := provider.Analyze(ctx, analysisReq)
	if err != nil {
		printStepError("Request failed")
		printFinal("❌", "LLM request failed")
		fmt.Printf("   Error: %v\n", err)
		fmt.Println("\n   💡 Check your API key in ~/.commit-tool/.env")
		if logger != nil {
			logger.LogError(err)
		}
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	printSuccess(fmt.Sprintf("Analysis complete"))

	// Log LLM response
	if logger != nil {
		logger.LogLLMResponse(0, len(plan.Commits))
	}

	// Validate and fix plan (merges overlapping commits, truncates long messages)
	printStep("📋", "Planning commits...")

	validator := planner.NewValidator(gitRoot, repoConfig, files)
	plan, validationResult := validator.ValidateAndFix(plan)

	// Log validation
	if logger != nil {
		var errorStrings []string
		for _, e := range validationResult.Errors {
			errorStrings = append(errorStrings, e.Error())
		}
		logger.LogPlanValidated(validationResult.Valid, errorStrings)
	}

	if !validationResult.Valid {
		printStepError("Validation failed")
		for _, e := range validationResult.Errors {
			fmt.Printf("   • %s\n", e.Error())
		}
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	// Filter sensitive files
	filteredFiles := planner.FilterSensitiveFiles(plan)
	if len(filteredFiles) > 0 {
		printWarning(fmt.Sprintf("Excluded %d sensitive files: %v", len(filteredFiles), filteredFiles))
	}

	if len(plan.Commits) == 0 {
		printFinal("❌", "No commits to create")
		fmt.Println("   All changes were filtered out.")
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		return result
	}

	printSuccess(fmt.Sprintf("%d commits planned", len(plan.Commits)))

	if flags.verbose {
		for i, c := range plan.Commits {
			var msg string
			if c.Scope != nil && *c.Scope != "" {
				msg = fmt.Sprintf("%s(%s): %s", c.Type, *c.Scope, c.Message)
			} else {
				msg = fmt.Sprintf("%s: %s", c.Type, c.Message)
			}
			printVerbose(fmt.Sprintf("  %d. %s", i+1, msg))
		}
	}

	// Execute plan
	if flags.dryRun {
		printStep("🚀", "Preview (dry-run)...")
	} else {
		printStep("🚀", "Executing commits...")
	}

	executor := planner.NewExecutor(gitRoot, flags.dryRun)

	executed, err := executor.Execute(plan, func(current, total int, commit types.PlannedCommit) {
		var msg string
		if commit.Scope != nil && *commit.Scope != "" {
			msg = fmt.Sprintf("%s(%s): %s", commit.Type, *commit.Scope, commit.Message)
		} else {
			msg = fmt.Sprintf("%s: %s", commit.Type, commit.Message)
		}

		if current == 1 {
			fmt.Printf("   ┌─ [%d/%d] %s\n", current, total, msg)
		} else if current == total {
			fmt.Printf("   └─ [%d/%d] %s\n", current, total, msg)
		} else {
			fmt.Printf("   ├─ [%d/%d] %s\n", current, total, msg)
		}

		for _, f := range commit.Files {
			fmt.Printf("   │  └─ %s\n", f)
		}
	})

	if err != nil {
		printError("Execution failed", err)
		if logger != nil {
			logger.LogError(err)
		}
		result.ExitCode = 1
		result.Duration = time.Since(startTime)
		result.CommitsCreated = executed
		return result
	}

	// Log commits
	if logger != nil {
		for _, c := range executed {
			logger.LogCommitExecuted(c.Hash, c.Message, c.Files)
		}
	}

	// Print final summary
	if flags.dryRun {
		printFinal("✅", fmt.Sprintf("Would create %d commits (dry-run)", len(executed)))
	} else {
		printFinal("✅", fmt.Sprintf("Created %d commits", len(executed)))
	}

	if flags.verbose && logger != nil {
		fmt.Printf("\n📝 Execution logged: %s\n", logger.Path())
	}

	result.Duration = time.Since(startTime)
	result.CommitsCreated = executed
	return result
}

func handleInteractive(flags flags) int {
	cwd, err := os.Getwd()
	if err != nil {
		printError("Failed to get current directory", err)
		return 1
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		printError("Not a git repository", err)
		return 1
	}

	// Run the interactive wizard
	completed, err := interactive.Run(interactive.Config{
		GitRoot: gitRoot,
		Force:   flags.force,
	})

	if err != nil {
		// Check if it's a pushed commit error
		if _, ok := err.(*interactive.PushedCommitError); ok {
			printStepError("Rebase includes pushed commits")
			printFinal("❌", "Cannot rebase pushed commits")
			fmt.Println("\n   Some commits in this rebase have been pushed to origin.")
			fmt.Println("   Rebasing will require force-push to sync with remote.")
			fmt.Println("\n   Use -i --force to proceed.")
			return 1
		}
		printError("Interactive rebase failed", err)
		return 1
	}

	if completed {
		printFinal("✅", "Rebase completed successfully")
	} else {
		fmt.Println("Cancelled.")
	}

	return 0
}

func handleDiff(flags flags) int {
	cwd, err := os.Getwd()
	if err != nil {
		printError("Failed to get current directory", err)
		return 1
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		printError("Not a git repository", err)
		return 1
	}

	// Load config
	printStep("🔧", "Loading config...")
	userConfig, err := config.LoadUserConfig()
	if err != nil {
		handleConfigError(err)
		return 1
	}

	if flags.provider != "" {
		userConfig.Provider = flags.provider
	}
	printSuccess(fmt.Sprintf("Provider: %s", userConfig.Provider))

	// Create LLM provider
	provider, err := llm.NewProvider(userConfig)
	if err != nil {
		printError("Failed to create LLM provider", err)
		return 1
	}

	// Analyze the diff
	printStep("📂", fmt.Sprintf("Analyzing: %s", flags.diffFile))

	diffAnalyzer := analyzer.NewDiffAnalyzer(gitRoot)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	printProgress(fmt.Sprintf("Sending to %s...", provider.Model()))

	analysis, err := diffAnalyzer.Analyze(ctx, flags.diffFile, flags.diffFrom, flags.diffTo, provider)
	if err != nil {
		printError("Analysis failed", err)
		return 1
	}

	printFinal("🤖", "Analysis:")
	fmt.Println()
	fmt.Println(analysis)
	fmt.Println()

	return 0
}

func handleReverse(gitRoot string, force, verbose bool) int {
	printStep("🔄", "Reversing HEAD commit...")

	reverser := git.NewReverser(gitRoot)

	// Check if commit was pushed
	pushed, _ := reverser.WasPushed()
	if pushed && !force {
		printStepError("Commit has been pushed")
		printFinal("❌", "Cannot reverse pushed commit")
		fmt.Println("\n   HEAD commit has been pushed to origin.")
		fmt.Println("   Reversing will require force-push to sync with remote.")
		fmt.Println("\n   Use --reverse --force to proceed.")
		return 1
	}

	if err := reverser.Reverse(force); err != nil {
		printError("Failed to reverse", err)
		return 1
	}

	printFinal("✅", "Reversed HEAD commit")
	fmt.Println("   Changes are now uncommitted in your working directory.")

	if pushed {
		printWarning("You will need to force-push after re-committing.")
	}

	return 0
}

func handleSetConfig(setting string) int {
	parts := strings.SplitN(setting, "=", 2)
	if len(parts) != 2 {
		fmt.Printf("Invalid format. Use: commit --set key=value\n")
		return 1
	}

	key, value := parts[0], parts[1]

	// Map user-friendly names to env vars
	var envKey string
	switch key {
	case "defaultMode":
		if value != "smart" && value != "single" {
			fmt.Printf("Invalid value for defaultMode. Use: smart or single\n")
			return 1
		}
		envKey = "COMMIT_DEFAULT_MODE"
	default:
		fmt.Printf("Unknown config key: %s\n", key)
		fmt.Println("Available keys: defaultMode")
		return 1
	}

	if err := config.SetConfigValue(envKey, value); err != nil {
		fmt.Printf("Failed to set config: %v\n", err)
		return 1
	}

	fmt.Printf("Set %s=%s\n", key, value)
	return 0
}

func handleConfigError(err error) {
	switch e := err.(type) {
	case *config.ConfigNotFoundError:
		printStepError("No config file found")
		printFinal("❌", "Configuration required")
		fmt.Println()
		fmt.Println("   Edit your config file to get started:")
		fmt.Println("   ~/.commit-tool/.env")
		fmt.Println()
		fmt.Println("   Set COMMIT_PROVIDER to one of: anthropic, openai, grok, gemini, azure-foundry")
		fmt.Println("   Then add the corresponding API key.")
		fmt.Println()
		fmt.Println("   📖 Documentation: https://github.com/dsswift/commit#configuration")

		// Try to create default config
		config.EnsureConfigDir()
		config.CreateDefaultConfig()

	case *config.ProviderNotConfiguredError:
		printStepError("No provider configured")
		printFinal("❌", "Configuration required")
		fmt.Println()
		fmt.Println("   Edit ~/.commit-tool/.env and set COMMIT_PROVIDER")
		fmt.Println()
		fmt.Println("   Supported providers: anthropic, openai, grok, gemini, azure-foundry")

	case *config.InvalidProviderError:
		printStepError(fmt.Sprintf("Invalid provider: %s", e.Provider))
		printFinal("❌", "Configuration error")
		fmt.Println()
		fmt.Printf("   Provider %q is not supported.\n", e.Provider)
		fmt.Println("   Supported providers: anthropic, openai, grok, gemini, azure-foundry")

	case *config.MissingAPIKeyError:
		printStepError(fmt.Sprintf("Missing %s", e.EnvVar))
		printFinal("❌", "Configuration error")
		fmt.Println()
		fmt.Printf("   Provider %q requires %s to be set.\n", e.Provider, e.EnvVar)
		fmt.Println("   Edit ~/.commit-tool/.env to add your API key.")

	case *config.InvalidDefaultModeError:
		printStepError(fmt.Sprintf("Invalid default mode: %s", e.Mode))
		printFinal("❌", "Configuration error")
		fmt.Println()
		fmt.Printf("   Default mode %q is not valid.\n", e.Mode)
		fmt.Println("   Use: smart or single")

	default:
		printError("Failed to load config", err)
	}
}

// Console output helpers

func printStep(emoji, message string) {
	fmt.Printf("\n%s %s\n", emoji, message)
}

func printSuccess(message string) {
	fmt.Printf("   ✓ %s\n", message)
}

func printStepError(message string) {
	fmt.Printf("   ✗ %s\n", message)
}

func printProgress(message string) {
	fmt.Printf("   ⋯ %s\n", message)
}

func printVerbose(message string) {
	fmt.Printf("   │ %s\n", message)
}

func printWarning(message string) {
	fmt.Printf("   ⚠️  %s\n", message)
}

func printError(message string, err error) {
	fmt.Printf("   ✗ %s: %v\n", message, err)
}

func printFinal(emoji, message string) {
	fmt.Printf("\n%s %s\n", emoji, message)
}

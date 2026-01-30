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
	"github.com/dsswift/commit/internal/llm"
	"github.com/dsswift/commit/internal/logging"
	"github.com/dsswift/commit/internal/planner"
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
		return 0
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

	return result.ExitCode
}

type flags struct {
	staged    bool
	dryRun    bool
	verbose   bool
	reverse   bool
	force     bool
	version   bool
	provider  string
}

func parseFlags() flags {
	f := flags{}

	flag.BoolVar(&f.staged, "staged", false, "Only commit staged files")
	flag.BoolVar(&f.dryRun, "dry-run", false, "Preview commits without creating them")
	flag.BoolVar(&f.verbose, "v", false, "Verbose output")
	flag.BoolVar(&f.verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&f.reverse, "reverse", false, "Reverse HEAD commit into uncommitted changes")
	flag.BoolVar(&f.force, "force", false, "Force operation (for --reverse on pushed commits)")
	flag.BoolVar(&f.version, "version", false, "Print version")
	flag.StringVar(&f.provider, "provider", "", "Override LLM provider")

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
		var scopeNames []string
		for _, s := range repoConfig.Scopes {
			scopeNames = append(scopeNames, s.Scope)
		}
		printSuccess(fmt.Sprintf("Scopes: %s (from .commit.json)", strings.Join(scopeNames, ", ")))
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

	// Validate plan
	printStep("📋", "Planning commits...")

	validator := planner.NewValidator(gitRoot, repoConfig, files)
	validationResult := validator.Validate(plan)

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

package ghsummon

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

const cmdName = "ghsummon"

// Run the ghsummon
func Run(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
	log.SetOutput(errStream)
	fs := flag.NewFlagSet(
		fmt.Sprintf("%s (v%s rev:%s)", cmdName, version, revision), flag.ContinueOnError)
	fs.SetOutput(errStream)
	ver := fs.Bool("version", false, "display version")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if *ver {
		return printVersion(outStream)
	}

	return run(ctx, outStream, errStream)
}

func run(ctx context.Context, outStream, errStream io.Writer) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is not set")
	}

	// Configure git to use the token for HTTPS operations (e.g. git fetch).
	if err := configureGitToken(ctx, token); err != nil {
		return fmt.Errorf("failed to configure git auth: %w", err)
	}

	ownerRepo := os.Getenv("GITHUB_REPOSITORY")
	if ownerRepo == "" {
		return fmt.Errorf("GITHUB_REPOSITORY is not set")
	}

	// Use GHSUMMON_BASE_SHA or GITHUB_EVENT_BEFORE for multi-commit push support
	baseSHA := os.Getenv("GHSUMMON_BASE_SHA")
	if baseSHA == "" {
		baseSHA = os.Getenv("GITHUB_EVENT_BEFORE")
	}

	// Detect @copilot prompts from git diff
	prompts, err := detectPrompts(ctx, baseSHA)
	if err != nil {
		return fmt.Errorf("failed to detect prompts: %w", err)
	}
	if len(prompts) == 0 {
		log.Println("no @copilot prompts detected")
		return setOutputs(0, nil)
	}

	gh, err := newGHClient(ctx, token, ownerRepo)
	if err != nil {
		return err
	}

	defaultBranch, err := gh.getDefaultBranch(ctx)
	if err != nil {
		return err
	}

	// Group prompts by file path — per-file exclusion per SKETCH.md
	filePrompts := make(map[string][]Prompt)
	for _, p := range prompts {
		filePrompts[p.FilePath] = append(filePrompts[p.FilePath], p)
	}

	var prNumbers []int
	for filePath, fps := range filePrompts {
		branch := BranchName(filePath)

		// Exclusion control: skip if branch already exists
		exists, err := gh.branchExists(ctx, branch)
		if err != nil {
			return err
		}
		if exists {
			log.Printf("skipping %s: branch %s already exists (work in progress)\n", filePath, branch)
			continue
		}

		// Create empty commit and branch
		commitMsg := fmt.Sprintf("ghsummon: %s", filePath)
		if err := gh.createEmptyCommitAndBranch(ctx, defaultBranch, branch, commitMsg); err != nil {
			return err
		}
		log.Printf("created branch: %s\n", branch)

		// Build consolidated prompt for all @copilot directives in this file
		prTitle := fmt.Sprintf("ghsummon: %s", filePath)
		prBody := buildPRBody(fps[0])
		comment := buildCopilotComment(fps[0])
		if len(fps) > 1 {
			prBody = buildMultiPRBody(filePath, fps)
			comment = buildMultiCopilotComment(filePath, fps)
		}

		// Create PR
		prNumber, prNodeID, err := gh.createPR(ctx, defaultBranch, branch, prTitle, prBody)
		if err != nil {
			return err
		}
		log.Printf("created PR #%d for %s\n", prNumber, filePath)

		// Assign copilot via GraphQL
		if err := gh.assignCopilot(ctx, prNodeID); err != nil {
			return err
		}
		log.Printf("assigned copilot to PR #%d\n", prNumber)

		// Post @copilot comment
		if err := gh.postCopilotComment(ctx, prNumber, comment); err != nil {
			return err
		}
		log.Printf("posted @copilot comment on PR #%d\n", prNumber)

		prNumbers = append(prNumbers, prNumber)
	}

	return setOutputs(len(prNumbers), prNumbers)
}

// setOutputs writes results to GITHUB_OUTPUT if available.
func setOutputs(count int, prNumbers []int) error {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_OUTPUT: %w", err)
	}
	defer f.Close()

	nums := make([]string, len(prNumbers))
	for i, n := range prNumbers {
		nums[i] = strconv.Itoa(n)
	}

	fmt.Fprintf(f, "pr_count=%d\n", count)
	fmt.Fprintf(f, "pr_numbers=%s\n", strings.Join(nums, ","))
	return nil
}

func printVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "%s v%s (rev:%s)\n", cmdName, version, revision)
	return err
}

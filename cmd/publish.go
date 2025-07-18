package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
)

const (
	DefaultSubstackURL = "<your Substack URL here>/post/new"
	SubstackURLEnvVar  = "SUBSTACK_URL"
)

type PRStatusResponse struct {
	CurrentBranch PRBranch `json:"currentBranch"`
}

type PRBranch struct {
	StatusCheckRollup []StatusCheck `json:"statusCheckRollup"`
	Reviews           []struct {
		State string `json:"state"`
	} `json:"reviews"`
}

type StatusCheck struct {
	Context string `json:"context"`
	State   string `json:"state"`
}

type PRStatusEnum int

const (
	StatusPending PRStatusEnum = iota
	StatusSuccess
	StatusFailure
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Create or manage a PR for the current branch",
	Long: `Create a pull request for the current branch and monitor its status.
Errors if on main branch or a branch already merged into main.
Waits for checks to pass and can be interrupted at any time.`,
	RunE: publishPR,
}

func publishPR(cmd *cobra.Command, args []string) error {
	// Validate we're in the writing directory
	if err := validateWritingDirectory(); err != nil {
		return stacktrace.Propagate(err, "directory validation failed")
	}

	// Get current branch
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return stacktrace.Propagate(err, "failed to get current branch")
	}

	// Check if on main branch
	if currentBranch == "main" {
		return stacktrace.NewError("cannot publish from main branch")
	}

	// Check if branch is already merged into main
	merged, err := isBranchMerged(currentBranch)
	if err != nil {
		return stacktrace.Propagate(err, "failed to check if branch is merged")
	}
	if merged {
		return stacktrace.NewError("branch '%s' is already merged into main", currentBranch)
	}

	// Check if PR already exists
	prURL, err := getPRForBranch(currentBranch)
	if err != nil {
		return stacktrace.Propagate(err, "failed to check for existing PR")
	}

	if prURL == "" {
		// Create new PR
		fmt.Printf("Creating PR for branch '%s'...\n", currentBranch)
		prURL, err = createPR(currentBranch)
		if err != nil {
			return stacktrace.Propagate(err, "failed to create PR")
		}
		fmt.Printf("PR created: %s\n", prURL)
	} else {
		fmt.Printf("Found existing PR: %s\n", prURL)
	}

	// Monitor PR status
	fmt.Println("Monitoring PR status...")
	return monitorPRStatus(currentBranch)
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", stacktrace.Propagate(err, "failed to get current branch")
	}
	return strings.TrimSpace(string(output)), nil
}

func isBranchMerged(branch string) (bool, error) {
	cmd := exec.Command("git", "branch", "--merged", "main")
	output, err := cmd.Output()
	if err != nil {
		return false, stacktrace.Propagate(err, "failed to check merged branches")
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Remove * prefix for current branch
		if strings.HasPrefix(line, "*") {
			line = strings.TrimSpace(line[1:])
		}
		if line == branch {
			return true, nil
		}
	}
	return false, nil
}

func getPRForBranch(branch string) (string, error) {
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "url")
	output, err := cmd.Output()
	if err != nil {
		// If gh pr view fails, likely no PR exists
		return "", nil
	}

	var pr struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(output, &pr); err != nil {
		return "", stacktrace.Propagate(err, "failed to parse PR JSON")
	}

	return pr.URL, nil
}

func createPR(branch string) (string, error) {
	cmd := exec.Command("gh", "pr", "create", "--title", branch, "--body", "")
	output, err := cmd.Output()
	if err != nil {
		return "", stacktrace.Propagate(err, "failed to create PR")
	}
	return strings.TrimSpace(string(output)), nil
}

func monitorPRStatus(branch string) error {
	// Set up interrupt handler
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Waiting for checks to pass (Ctrl+C to stop monitoring)...")

	// Check immediately first
	if checkPRStatusOnce(branch) {
		return handleSuccessfulChecks(branch)
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c:
			fmt.Println("\nMonitoring interrupted by user")
			return nil
		case <-ticker.C:
			if checkPRStatusOnce(branch) {
				return handleSuccessfulChecks(branch)
			}
		}
	}
}

func checkPRStatusOnce(branch string) bool {
	status, err := getPRStatus(branch)
	if err != nil {
		fmt.Printf("Error getting PR status: %v\n", err)
		return false // Continue monitoring on error
	}

	overallStatus := getOverallStatus(status.StatusCheckRollup)

	switch overallStatus {
	case StatusSuccess:
		fmt.Println("All checks passed! ✅")
		return true
	case StatusFailure:
		fmt.Println("Some checks failed ❌")
		return false
	case StatusPending:
		fmt.Println("Checks are still running...")
		return false
	default:
		return false
	}
}

func getOverallStatus(statusChecks []StatusCheck) PRStatusEnum {
	if len(statusChecks) == 0 {
		return StatusPending
	}

	for _, check := range statusChecks {
		switch check.State {
		case "FAILURE", "ERROR":
			return StatusFailure
		case "PENDING":
			return StatusPending
		}
	}

	return StatusSuccess
}

func getPRStatus(branch string) (*PRBranch, error) {
	cmd := exec.Command("gh", "pr", "status", "--json", "statusCheckRollup,reviews")
	output, err := cmd.Output()
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to get PR status")
	}

	var statusResponse PRStatusResponse
	if err := json.Unmarshal(output, &statusResponse); err != nil {
		return nil, stacktrace.Propagate(err, "failed to parse PR status JSON")
	}

	return &statusResponse.CurrentBranch, nil
}

func validateWritingDirectory() error {
	// Get current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "failed to get current working directory")
	}

	// Get writing directory from environment variable
	writingDir := os.Getenv(WritingDirEnvVar)
	if writingDir == "" {
		return stacktrace.NewError("writing directory not configured: %s environment variable not set", WritingDirEnvVar)
	}

	// Convert to absolute paths for comparison
	absWritingDir, err := filepath.Abs(writingDir)
	if err != nil {
		return stacktrace.Propagate(err, "failed to get absolute path for writing directory")
	}

	absCurrentDir, err := filepath.Abs(currentDir)
	if err != nil {
		return stacktrace.Propagate(err, "failed to get absolute path for current directory")
	}

	// Check if current directory is within writing directory
	relPath, err := filepath.Rel(absWritingDir, absCurrentDir)
	if err != nil {
		return stacktrace.Propagate(err, "failed to calculate relative path")
	}

	// If relative path starts with "..", we're outside the writing directory
	if strings.HasPrefix(relPath, "..") {
		return stacktrace.NewError("must run publish command from within the writing directory (%s) or one of its subdirectories", absWritingDir)
	}

	return nil
}

func getSubstackURL() string {
	// Check if env file exists
	if _, err := os.Stat(EnvFilename); os.IsNotExist(err) {
		return DefaultSubstackURL
	}

	// Load env file using godotenv
	envVars, err := godotenv.Read(EnvFilename)
	if err != nil {
		return DefaultSubstackURL
	}

	// Get SUBSTACK_URL from env vars
	if url, exists := envVars[SubstackURLEnvVar]; exists && url != "" {
		return url + "/publish/post?type=newsletter"
	}

	return DefaultSubstackURL
}

func handleSuccessfulChecks(branch string) error {
	fmt.Println("Merging PR and cleaning up...")

	// Merge PR and delete remote branch
	if err := mergePR(branch); err != nil {
		return stacktrace.Propagate(err, "failed to merge PR")
	}

	// Switch to main branch
	if err := switchToMain(); err != nil {
		return stacktrace.Propagate(err, "failed to switch to main branch")
	}

	// Pull latest changes
	if err := pullMain(); err != nil {
		return stacktrace.Propagate(err, "failed to pull main branch")
	}

	// Delete local branch
	if err := deleteLocalBranch(branch); err != nil {
		return stacktrace.Propagate(err, "failed to delete local branch")
	}

	// Find the post directory that was added in this branch
	postDir, err := getAddedPostDirectory(branch)
	if err != nil {
		return stacktrace.Propagate(err, "failed to find added post directory")
	}

	// Open post in Chrome
	if err := openPostInChrome(postDir); err != nil {
		return stacktrace.Propagate(err, "failed to open post in Chrome")
	}

	// Print Substack URL
	substackURL := getSubstackURL()
	fmt.Println("\nPaste the rendered Markdown output into:")
	fmt.Println(substackURL)

	// Show tip if using placeholder URL
	if substackURL == DefaultSubstackURL {
		fmt.Printf("\n💡 Tip: Create a %s file with %s=https://yourname.substack.com to get a working link.\n", EnvFilename, SubstackURLEnvVar)
	}

	return nil
}

func mergePR(branch string) error {
	cmd := exec.Command("gh", "pr", "merge", branch, "--merge", "--delete-branch")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return stacktrace.NewError("failed to merge PR: %s", string(output))
	}
	fmt.Println("PR merged and remote branch deleted")
	return nil
}

func switchToMain() error {
	cmd := exec.Command("git", "checkout", "main")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return stacktrace.NewError("failed to checkout main: %s", string(output))
	}
	fmt.Println("Switched to main branch")
	return nil
}

func pullMain() error {
	cmd := exec.Command("git", "pull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return stacktrace.NewError("failed to pull main: %s", string(output))
	}
	fmt.Println("Pulled latest changes")
	return nil
}

func deleteLocalBranch(branch string) error {
	// First check if the branch exists
	cmd := exec.Command("git", "branch", "--list", branch)
	output, err := cmd.Output()
	if err != nil {
		return stacktrace.NewError("failed to check if branch exists: %s", string(output))
	}

	// If no output, branch doesn't exist
	if strings.TrimSpace(string(output)) == "" {
		fmt.Printf("Local branch '%s' already deleted\n", branch)
		return nil
	}

	// Branch exists, try to delete it
	cmd = exec.Command("git", "branch", "-d", branch)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return stacktrace.NewError("failed to delete local branch: %s", string(output))
	}
	fmt.Printf("Deleted local branch '%s'\n", branch)
	return nil
}

func getAddedPostDirectory(branch string) (string, error) {
	// Get files that were added in this branch compared to main
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=A", "main...HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", stacktrace.Propagate(err, "failed to get added files")
	}

	var addedPostDirs []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		file := scanner.Text()
		if strings.HasSuffix(file, "/post.md") {
			dir := filepath.Dir(file)
			// Skip TEMPLATE directory
			if dir != "TEMPLATE" {
				addedPostDirs = append(addedPostDirs, dir)
			}
		}
	}

	if len(addedPostDirs) == 0 {
		return "", stacktrace.NewError("no post.md files were added in this branch")
	}

	if len(addedPostDirs) > 1 {
		return "", stacktrace.NewError("multiple post.md files were added in this branch: %v", addedPostDirs)
	}

	return addedPostDirs[0], nil
}

func openPostInChrome(postDir string) error {
	postPath := fmt.Sprintf("%s/post.md", postDir)
	cmd := exec.Command("open", "-a", "Google Chrome", postPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return stacktrace.NewError("failed to open post in Chrome: %s", string(output))
	}
	fmt.Printf("Opened %s in Chrome\n", postPath)
	return nil
}

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
)

type PRStatus struct {
	StatusCheckRollup string `json:"statusCheckRollup"`
	Reviews           []struct {
		State string `json:"state"`
	} `json:"reviews"`
}

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Create or manage a PR for the current branch",
	Long: `Create a pull request for the current branch and monitor its status.
Errors if on main branch or a branch already merged into main.
Waits for checks to pass and can be interrupted at any time.`,
	RunE: publishPR,
}

func publishPR(cmd *cobra.Command, args []string) error {
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

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	fmt.Println("Waiting for checks to pass (Ctrl+C to stop monitoring)...")

	for {
		select {
		case <-c:
			fmt.Println("\nMonitoring interrupted by user")
			return nil
		case <-ticker.C:
			status, err := getPRStatus(branch)
			if err != nil {
				fmt.Printf("Error getting PR status: %v\n", err)
				continue
			}

			fmt.Printf("Status: %s\n", status.StatusCheckRollup)
			
			if status.StatusCheckRollup == "SUCCESS" {
				fmt.Println("All checks passed! ✅")
				return nil
			} else if status.StatusCheckRollup == "FAILURE" {
				fmt.Println("Some checks failed ❌")
				return stacktrace.NewError("PR checks failed")
			}
			// Continue monitoring for PENDING or other statuses
		}
	}
}

func getPRStatus(branch string) (*PRStatus, error) {
	cmd := exec.Command("gh", "pr", "status", "--json", "statusCheckRollup,reviews")
	output, err := cmd.Output()
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to get PR status")
	}

	var statuses []PRStatus
	if err := json.Unmarshal(output, &statuses); err != nil {
		return nil, stacktrace.Propagate(err, "failed to parse PR status JSON")
	}

	if len(statuses) == 0 {
		return nil, stacktrace.NewError("no PR status found")
	}

	return &statuses[0], nil
}
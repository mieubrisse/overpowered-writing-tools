package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
)

const (
	MainBranchName  = "main"
	TemplateDirname = "TEMPLATE"
	PostFilename    = "post.md"
)

var addCmd = &cobra.Command{
	Use:   "add [name_words...]",
	Short: "Create a new post directory and branch",
	Long: `Create a new post by copying from TEMPLATE directory, creating a new Git branch,
and committing the initial files. Name words will be joined with hyphens.`,
	Args: cobra.MinimumNArgs(1),
	RunE: addPost,
}

func addPost(cmd *cobra.Command, args []string) error {
	// Get writing directory from environment variable
	writingRepoPath := os.Getenv(WritingDirEnvVar)
	if writingRepoPath == "" {
		return stacktrace.NewError("writing directory not configured: %s environment variable not set", WritingDirEnvVar)
	}

	nameWords := args
	if len(nameWords) == 0 {
		return stacktrace.NewError("post name must have at least one word")
	}

	// Join name words with hyphens
	postName := strings.Join(nameWords, "-")
	if postName == "" {
		return stacktrace.NewError("post name cannot be empty")
	}

	// Check if post name contains spaces (shouldn't happen with our join, but safety check)
	if strings.Contains(postName, " ") {
		return stacktrace.NewError("new post name cannot have spaces but was '%s'", postName)
	}

	postDirPath := filepath.Join(writingRepoPath, postName)

	// Check if directory already exists
	if _, err := os.Stat(postDirPath); err == nil {
		return stacktrace.NewError("can't create post; directory already exists: %s", postDirPath)
	}

	// Check if git branch already exists
	checkBranchCmd := exec.Command("git", "-C", writingRepoPath, "rev-parse", "--verify", postName)
	if err := checkBranchCmd.Run(); err == nil {
		return stacktrace.NewError("can't create post; git branch already exists: %s", postName)
	}

	// Change to writing repo directory
	originalDir, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "failed to get current directory")
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(writingRepoPath); err != nil {
		return stacktrace.Propagate(err, "couldn't cd to writing repo: %s", writingRepoPath)
	}

	// Checkout main branch
	checkoutMainCmd := exec.Command("git", "checkout", MainBranchName)
	checkoutMainCmd.Stdout = nil // Suppress output
	if err := checkoutMainCmd.Run(); err != nil {
		return stacktrace.Propagate(err, "couldn't check out main branch")
	}

	// Create and checkout new branch
	checkoutNewBranchCmd := exec.Command("git", "checkout", "-b", postName)
	checkoutNewBranchCmd.Stdout = nil // Suppress output
	if err := checkoutNewBranchCmd.Run(); err != nil {
		return stacktrace.Propagate(err, "failed to check out new branch: %s", postName)
	}

	// Copy template directory to new post directory
	copyCmd := exec.Command("cp", "-R", TemplateDirname, postName)
	if err := copyCmd.Run(); err != nil {
		return stacktrace.Propagate(err, "failed to create new post directory from template")
	}

	// Change to new post directory
	if err := os.Chdir(postName); err != nil {
		return stacktrace.Propagate(err, "couldn't cd to new directory: %s", postName)
	}

	// Add files to git
	addCmd := exec.Command("git", "add", ".")
	addCmd.Stdout = nil // Suppress output
	if err := addCmd.Run(); err != nil {
		return stacktrace.Propagate(err, "failed to add new files")
	}

	// Commit files
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Initial commit for %s", postName))
	commitCmd.Stdout = nil // Suppress output
	if err := commitCmd.Run(); err != nil {
		return stacktrace.Propagate(err, "failed to commit new files")
	}

	// Output the post directory path
	fmt.Println(postDirPath)
	return nil
}
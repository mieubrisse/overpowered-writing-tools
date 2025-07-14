package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
)

type PostEntry struct {
	Dir    string
	Branch string
}

type BranchDistance struct {
	Branch   string
	Distance int
}

var findCmd = &cobra.Command{
	Use:   "find [writing_repo_path] [search_terms...]",
	Short: "Find and select a post directory from any branch",
	Long: `Find post directories across all Git branches, sorted by commit distance from main,
and allow interactive selection with fzf.`,
	Args: cobra.MinimumNArgs(1),
	RunE: findPosts,
}

func findPosts(cmd *cobra.Command, args []string) error {
	writingRepoPath := args[0]
	searchTerms := strings.Join(args[1:], " ")

	if writingRepoPath == "" {
		return stacktrace.NewError("writing repo path cannot be empty")
	}

	// Check if the directory exists and is a git repo
	if _, err := os.Stat(writingRepoPath); os.IsNotExist(err) {
		return stacktrace.NewError("writing repo path does not exist: %s", writingRepoPath)
	}

	seenDirs := make(map[string]bool)
	branchMapping := make(map[string]string)
	var entries []string

	// Get main branch post directories first (they take precedence)
	mainPosts, err := getPostDirsFromBranch(writingRepoPath, "main")
	if err != nil {
		return stacktrace.Propagate(err, "failed to get posts from main branch")
	}

	for _, dir := range mainPosts {
		if !seenDirs[dir] {
			entries = append(entries, dir)
			branchMapping[dir] = "main"
			seenDirs[dir] = true
		}
	}

	// Get all non-main branches sorted by distance from main
	sortedBranches, err := getBranchesSortedByDistance(writingRepoPath)
	if err != nil {
		return stacktrace.Propagate(err, "failed to get sorted branches")
	}

	// Process each branch in order
	for _, branchDist := range sortedBranches {
		posts, err := getPostDirsFromBranch(writingRepoPath, branchDist.Branch)
		if err != nil {
			return stacktrace.Propagate(err, "failed to get posts from branch %s", branchDist.Branch)
		}

		for _, dir := range posts {
			if !seenDirs[dir] {
				entries = append(entries, dir)
				branchMapping[dir] = branchDist.Branch
				seenDirs[dir] = true
			}
		}
	}

	// Sort entries by last commit date
	sortedEntries, err := sortEntriesByCommitDate(writingRepoPath, entries, branchMapping)
	if err != nil {
		return stacktrace.Propagate(err, "failed to sort entries by commit date")
	}

	// Launch fzf for selection
	selection, err := runFzf(sortedEntries, searchTerms)
	if err != nil {
		return stacktrace.Propagate(err, "fzf selection failed")
	}

	if selection == "" {
		os.Exit(2) // User cancelled - exit with status 2
	}

	// Look up the branch for this selection
	branch, exists := branchMapping[selection]
	if !exists {
		return stacktrace.NewError("no branch mapping found for selection: %s", selection)
	}

	// Output the result
	fmt.Printf("%s %s\n", branch, selection)
	return nil
}

func getPostDirsFromBranch(repoPath, branch string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "ls-tree", "-r", "--name-only", branch)
	output, err := cmd.Output()
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to run git ls-tree for branch %s", branch)
	}

	var dirs []string
	postRegex := regexp.MustCompile(`/post\.md$`)
	
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		file := scanner.Text()
		if postRegex.MatchString(file) {
			dir := filepath.Dir(file)
			dirs = append(dirs, dir)
		}
	}

	return dirs, nil
}

func getBranchesSortedByDistance(repoPath string) ([]BranchDistance, error) {
	// Get all branches except main
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)", "--no-merged", "main")
	output, err := cmd.Output()
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to get branches")
	}

	var branches []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		branch := strings.TrimSpace(scanner.Text())
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	if len(branches) == 0 {
		return []BranchDistance{}, nil
	}

	// Calculate distances in parallel
	var wg sync.WaitGroup
	distances := make([]BranchDistance, len(branches))
	
	for i, branch := range branches {
		wg.Add(1)
		go func(idx int, branchName string) {
			defer wg.Done()
			
			cmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", fmt.Sprintf("main..%s", branchName))
			output, err := cmd.Output()
			
			distance := 999999 // Default for error cases
			if err == nil {
				if d, parseErr := strconv.Atoi(strings.TrimSpace(string(output))); parseErr == nil {
					distance = d
				}
			}
			
			distances[idx] = BranchDistance{
				Branch:   branchName,
				Distance: distance,
			}
		}(i, branch)
	}
	
	wg.Wait()

	// Sort by distance
	sort.Slice(distances, func(i, j int) bool {
		return distances[i].Distance < distances[j].Distance
	})

	return distances, nil
}

func sortEntriesByCommitDate(repoPath string, entries []string, branchMapping map[string]string) ([]string, error) {
	type entryWithTimestamp struct {
		dir       string
		timestamp int64
	}

	var entriesWithTimestamps []entryWithTimestamp
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, dir := range entries {
		wg.Add(1)
		go func(directory string) {
			defer wg.Done()
			
			branch := branchMapping[directory]
			cmd := exec.Command("git", "-C", repoPath, "log", "--max-count=1", "--format=%ct", branch, "--", directory)
			output, err := cmd.Output()
			
			timestamp := int64(0) // Default for error cases
			if err == nil {
				if ts, parseErr := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); parseErr == nil {
					timestamp = ts
				}
			}
			
			mu.Lock()
			entriesWithTimestamps = append(entriesWithTimestamps, entryWithTimestamp{
				dir:       directory,
				timestamp: timestamp,
			})
			mu.Unlock()
		}(dir)
	}
	
	wg.Wait()

	// Sort by timestamp (most recent first)
	sort.Slice(entriesWithTimestamps, func(i, j int) bool {
		return entriesWithTimestamps[i].timestamp > entriesWithTimestamps[j].timestamp
	})

	var sortedEntries []string
	for _, entry := range entriesWithTimestamps {
		sortedEntries = append(sortedEntries, entry.dir)
	}

	return sortedEntries, nil
}

func runFzf(entries []string, query string) (string, error) {
	cmd := exec.Command("fzf", "--query", query)
	
	// Create stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", stacktrace.Propagate(err, "failed to create stdin pipe for fzf")
	}
	
	// Create stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", stacktrace.Propagate(err, "failed to create stdout pipe for fzf")
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return "", stacktrace.Propagate(err, "failed to start fzf")
	}
	
	// Write entries to stdin
	for _, entry := range entries {
		if _, err := fmt.Fprintln(stdin, entry); err != nil {
			stdin.Close()
			return "", stacktrace.Propagate(err, "failed to write to fzf stdin")
		}
	}
	stdin.Close()
	
	// Read output
	output, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil && err.Error() != "EOF" {
		return "", stacktrace.Propagate(err, "failed to read fzf output")
	}
	
	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		// fzf returns exit code 1 when user cancels with ESC, exit code 130 when user cancels with Ctrl+C
		if exitError, ok := err.(*exec.ExitError); ok && (exitError.ExitCode() == 1 || exitError.ExitCode() == 130) {
			return "", nil // User cancelled
		}
		return "", stacktrace.Propagate(err, "fzf execution failed")
	}
	
	return strings.TrimSpace(output), nil
}
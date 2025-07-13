set -x 

# This script presents the user with the possible posts (directories) that could be
# switched to across all branches, allows the user to pick one with fzf, and 
# allows the user to 
set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"

usage() {
    echo "Usage: ${0} writing_repo_dirpath [query_term1] [query_term2]..." >&2
}

if [ "${#}" -eq 0 ]; then
    usage
    exit 1
fi

writing_repo_dirpath="${1}"; shift
if [ -z "${writing_repo_dirpath}" ]; then
    echo "Error: writing repo dirpath cannot be empty" >&2
    usage
    exit 1
fi

# Search terms can be empty; user will fill it in interactively
search_terms="${*}"

# Use associative arrays to store our data to get O(1) lookup times
declare -A seen_dirs
declare -A branch_mapping
declare -a entries

# Get main branch post directories first (they take precedence)
while IFS="" read -r file; do
    dir=$(dirname "$file")
    if [ -z "${seen_dirs[$dir]:-}" ]; then
        entries+=("$dir")
        branch_mapping["$dir"]="main"
        seen_dirs["$dir"]=1
    fi
done < <(git -C "${writing_repo_dirpath}" ls-tree -r --name-only main | grep '/post\.md$')

# Get all branches and sort them by commit distance from main (using batch processing)
declare -a sorted_branches
branches_list=($(git -C "${writing_repo_dirpath}" branch --format='%(refname:short)' --no-merged main))

# Build a single git command to get all distances at once
if [ ${#branches_list[@]} -gt 0 ]; then
    # Create a format string for git for-each-ref that includes distance calculation
    while IFS= read -r line; do
        sorted_branches+=("${line#* }")
    done < <(
        # Use git for-each-ref with parallel processing to get distances
        git -C "${writing_repo_dirpath}" for-each-ref --format='%(refname:short)' refs/heads/ | \
        grep -v '^main$' | \
        xargs -P 0 -I {} sh -c 'distance=$(git -C "'"${writing_repo_dirpath}"'" rev-list --count "main..{}" 2>/dev/null || echo "999999"); echo "$distance {}"' | \
        sort -n
    )
else
    sorted_branches=()
fi

# Process all branches using git ls-tree to get all files in each branch
for branch in "${sorted_branches[@]}"; do
    while IFS= read -r file; do
        dir=$(dirname "$file")
        if [[ -z "${seen_dirs[$dir]:-}" ]]; then
            entries+=("$dir")
            branch_mapping["$dir"]="$branch"
            seen_dirs["$dir"]=1
        fi
    done < <(git -C "${writing_repo_dirpath}" ls-tree -r --name-only "$branch" | grep '/post\.md$')
done

# Sort entries by last commit date (most recent first) - optimized version
declare -a sorted_entries
while IFS= read -r dir; do
    sorted_entries+=("$dir")
done < <(
    for dir in "${entries[@]}"; do
        branch="${branch_mapping[$dir]}"
        # Use git log with --max-count=1 and specific path for faster lookup
        timestamp=$(git -C "${writing_repo_dirpath}" log --max-count=1 --format="%ct" "$branch" -- "$dir" 2>/dev/null || echo "0")
        echo "$timestamp $dir"
    done | sort -rn | cut -d' ' -f2-
)

# Launch fzf with sorted entries
selection=$(printf '%s\n' "${sorted_entries[@]}" | fzf --query="${search_terms}") # Note that we intentionally don't use $@ so that we get a single string with space separator

[ -z "$selection" ] && return

# Look up which branch to use for this directory
branch="${branch_mapping[$selection]}"

if [ -z "$branch" ]; then
    echo "Error: There was no branch mapping for selection: ${selection}" >&2
    return 1
fi

# This will fail if the post directory or branch ever have a space, but I'll deal with that edge case when/if it happens
echo "${branch} ${selection}"

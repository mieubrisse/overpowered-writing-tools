# This script presents the user with the possible posts (directories) that could be
# switched to across all branches, allows the user to pick one with fzf, and 
# allows the user to 
set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"

usage() {
    echo "Usage: ${0} writing_repo_dirpath query_term1 [query_term2]..." >&2
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

search_terms="${*}"
if [ -z "${search_terms}" ]; then
    echo "Error: at least one search term must be provided" >&2
    usage
    exit 1
fi

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

# Get all branches at once and process them
branches=($(git -C "${writing_repo_dirpath}" branch --format='%(refname:short)' --no-merged main))

# Process all branches using git show instead of ls-tree
for branch in "${branches[@]}"; do
    while IFS= read -r file; do
        dir=$(dirname "$file")
        if [[ -z "${seen_dirs[$dir]:-}" ]]; then
            entries+=("$dir")
            branch_mapping["$dir"]="$branch"
            seen_dirs["$dir"]=1
        fi
    done < <(git -C "${writing_repo_dirpath}" show --name-only --pretty=format: "refs/heads/$branch" | grep '/post\.md$')
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

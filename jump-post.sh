# Opens in Vim a post inside the given blog post repo
# Expects the first argument to be the post repo
# You'll likely want to add an alias for this in your .bashrc

set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"

if [ -z "${#}" ]; then
    echo "Usage: jump_post /your/blog/post/repo " >&2
    return 1
fi
blog_repo="${1}"; shift
if ! [ -d "${blog_repo}" ]; then
    echo "Error: given blog post repo '${blog_repo}' isn't a directory" >&2
    return 1
fi
query="${1}"

# Use associative arrays to store our data to get O(1) lookup times
declare -A seen_dirs
declare -A branch_mapping
declare -a entries

# Get main branch post directories first (they take precedence)
while IFS= read -r file; do
    dir=$(dirname "$file")
    if [ -z "${seen_dirs[$dir]:-}" ]; then
        entries+=("$dir")
        branch_mapping["$dir"]="main"
        seen_dirs["$dir"]=1
    fi
done < <(git -C "$blog_repo" ls-tree -r --name-only main | grep '/post\.md$')

# Get all branches at once and process them
branches=($(git -C "$blog_repo" branch --format='%(refname:short)' --no-merged main))

# Process all branches using git show instead of ls-tree
for branch in "${branches[@]}"; do
    while IFS= read -r file; do
        dir=$(dirname "$file")
        if [[ -z "${seen_dirs[$dir]:-}" ]]; then
            entries+=("$dir")
            branch_mapping["$dir"]="$branch"
            seen_dirs["$dir"]=1
        fi
    done < <(git -C "$blog_repo" show --name-only --pretty=format: "refs/heads/$branch" | grep '/post\.md$')
done

# Sort entries by last commit date (most recent first) - optimized version
declare -a sorted_entries
while IFS= read -r dir; do
    sorted_entries+=("$dir")
done < <(
    for dir in "${entries[@]}"; do
        branch="${branch_mapping[$dir]}"
        # Use git log with --max-count=1 and specific path for faster lookup
        timestamp=$(git -C "$blog_repo" log --max-count=1 --format="%ct" "$branch" -- "$dir" 2>/dev/null || echo "0")
        echo "$timestamp $dir"
    done | sort -rn | cut -d' ' -f2-
)

# Launch fzf with sorted entries
selection=$(printf '%s\n' "${sorted_entries[@]}" | fzf --query="${*}") # Note that we intentionally don't use $@ so that we get a single string with space separator

[ -z "$selection" ] && return

# Look up which branch to use for this directory
branch="${branch_mapping[$selection]}"

if [ -z "$branch" ]; then
    echo "Error: There was no branch mapping for selection: ${selection}" >&2
    return 1
fi

if ! git -C "$blog_repo" switch "$branch" >/dev/null; then
    echo "Error: Couldn't change repo to branch: ${branch}" >&2
    return 1
fi

cd "$blog_repo/$selection" && vim post.md
}

# Quick helper to summon up a new post inside this repo
# Expects the first argument to be the post repo, and the second argument to be the name of the post branch
# You'll likely want to add an alias for this in your .bashrc
function new_post() {
if [ "${#}" -eq 0 ]; then
    echo "Usage: jump_post /your/blog/post/repo post" >&2
    return 1
fi

blog_repo="${1}"; shift
if ! [ -d "${blog_repo}" ]; then
    echo "Error: given blog post repo '${blog_repo}' isn't a directory" >&2
    return 1
fi

post_name_spaces="${*}"
post_name="${post_name_spaces// /-}"
if [ -z "${post_name}" ]; then
    echo "Error: post name cannot be empty" >&2
    return 1
fi
if [[ "${post_name}" == *" "* ]]; then
    echo "New post name cannot have spaces" >&2
    return 1
fi


cd "${code_dirpath}/personal-writing"
if [ -d "${post_name}" ]; then
    echo "Directory '${post_name}' already exists" >&2
    return 1
fi
if git rev-parse --verify "${post_name}" &> /dev/null; then
    echo "Git branch '${post_name}' already exists" >&2
    return 1
fi

cp -R TEMPLATE "${post_name}"
git checkout -b "${post_name}"
cd "${post_name}"
git add .
git commit -m "Initial commit for ${post_name}"
vim "post.md"

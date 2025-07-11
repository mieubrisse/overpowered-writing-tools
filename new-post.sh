# Quick helper to summon up a new post inside the writing repo
# Expects arguments to be fragments of the new post's name (which will get joined
# with "-")
set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"

MAIN_BRANCH_NAME="main"  # Prob worth making this configurable
TEMPLATE_DIRNAME="TEMPLATE"
POST_FILENAME="post.md"

usage() {
    echo "Usage: ${0} writing_repo_dirpath name_word1 [name_word2]..." >&2
    echo "" >&2
    echo "Example: ${0} my new post" >&2
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

if [ "${#}" -eq 0 ]; then
    echo "Error: Post name must have at least one word" >&2
    usage
    exit 1
fi

post_name_spaces="${*}"
post_name="${post_name_spaces// /-}"
if [ -z "${post_name}" ]; then
    echo "Error: post name cannot be empty" >&2
    usage
    exit 1
fi
if [[ "${post_name}" == *" "* ]]; then
    echo "Error: New post name cannot have spaces but was '${post_name}'" >&2
    exit 1
fi

post_dirpath="${writing_repo_dirpath}/${post_name}"
if [ -d "${post_dirpath}" ]; then
    echo "Error: Can't create post; directory already exists: ${post_dirpath}" >&2
    exit 1
fi
if git rev-parse --verify "${post_name}" &> /dev/null; then
    echo "Error: Can't create post; git branch already exists: ${post_name}" >&2
    exit 1
fi

if ! cd "${WRITING_REPO_DIRPATH}"; then
    echo "Error: Couldn't cd to writing repo: ${WRITING_REPO_DIRPATH}" >&2
    exit 1
fi

if ! git checkout main >/dev/null; then
    echo "Error: Couldn't check out main branch" >&2
    exit 1
fi

if ! git checkout -b "${post_name}" >/dev/null; then
    echo "Error: Failed to check out new branch: ${post_name}" >&2
    exit 1
fi

if ! cp -R "${TEMPLATE_DIRNAME}" "${post_name}"; then
    echo "Error: Failed to create new post directory from template" >&2
    exit 1
fi

if ! cd "${post_name}"; then
    echo "Error: Couldn't cd to new directory: ${post_name}" >&2
    exit 1
fi

if ! git add . >/dev/null; then
    echo "Error: Failed to add new files" >&2
    exit 1
fi

if ! git commit -m "Initial commit for ${post_name}" >/dev/null; then
    echo "Error: Failed to commit new files" >&2
    exit 1
fi

echo "${post_dirpath}"

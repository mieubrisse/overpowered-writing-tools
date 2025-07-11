_WRITING_TOOLS_DIRPATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_FIND_POST_SCRIPTNAME="find-post.sh"

# Opens in Vim a post inside the given blog post repo
# Expects the first argument to be the post repo
# You'll likely want to add an alias for this in your .bashrc
find_post() {
    if [ -z "${WRITING_REPO_DIRPATH}" ]; then
        echo "Error: WRITING_REPO_DIRPATH var must point to your writing repo" >&2
        return 1
    fi

    if ! find_post_output="$(
        bash "${_WRITING_TOOLS_DIRPATH}/${_FIND_POST_SCRIPTNAME}" "${WRITING_REPO_DIRPATH}" "${@}"
    )"; then
        echo "Error: ${_FIND_POST_SCRIPTNAME} failed" >&2
        return 1
    fi

    read -r post_branch post_directory < <(echo "${find_post_output}")
    if [ -z "${post_branch}" ]; then
        echo "Error: ${_FIND_POST_SCRIPTNAME} returned an empty branch" >&2
        return 1
    fi
    if [ -z "${post_directory}" ]; then
        echo "Error: ${_FIND_POST_SCRIPTNAME} returned an empty directory" >&2
        return 1
    fi

    cd "${WRITING_REPO_DIRPATH}"

    if ! git checkout "${post_branch}"; then
        echo "Error: An error occurred checking out branch '${post_branch}'" >&2
        return 1
    fi

    if ! cd "${post_directory}"; then
        echo "Error: Failed to check out post directory '${post_directory}'" >&2
        return 1
    fi

    vim post.md
}

# TODO new_post

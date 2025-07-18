Overpowered Writing Tools
=========================
This repository provides a collection of tools to manage the writing repository described in An Overpowered Writing System (TODO LINK).

It expects a writing repo in the Overpowered Writing System format:

```
TEMPLATE/
    post.md
    images/
some-post/
    post.md
    images/
        image.png
some-other-post/
    post.md
    images/
        image.png
```

Installation
------------
1. Install `git` and [the Github CLI `gh`](https://cli.github.com/) if you haven't already
1. Download the appropriate `opwriting` binary for your OS/arch from [the releases page](https://github.com/mieubrisse/overpowered-writing-tools/releases)
1. Rename the binary to `opwriting` (remove the OS/arch information) and store it somewhere on your machine
1. Add the following to your `.bashrc`/`.zshrc`, replacing the `TODO`s with the appropriate values:
   ```bash
   export PATH=/where/your/opwriting/binary/lives  # Directory containing the 'opwriting' binary
   export WRITING_REPO_DIRPATH=/your/writing/repo  # Your writing repo
   eval "$(opwriting shell)"                       # Adds functions to manage the repo (see below)
   ```

Usage
-----
The `eval "$(opwriting shell)"` makes the following functions available. You might consider aliasing them (e.g. I use `alias pj="jump_post"`, `alias pn="new_post"`, etc.).

### jump_post
`jump_post [search_term] [search_term2]..` will:

1. Collect all post directories across all branches in `$WRITING_REPO_DIRPATH`
1. Present the user with a list, which is interactively filterable using [`fzf`](https://github.com/junegunn/fzf)
1. Upon selection, switch to the post's branch and cd to the post's directory

### edit_post
`edit_post [search_term] [search_term2]..` will do everything that `jump_post` does, plus open the `post.md` in the user's `$EDITOR`.

### new_post
`new_post post_word1 [post_word2]...` will:

1. Create a new branch on `$WRITING_REPO_DIRPATH` by joining the post word args with `-` (e.g. `new_post my new post` creates `my-new-post`)
1. Clone the `TEMPLATE` directory to create a new directory with the same name as the branch
1. Open `post.md` in the user's `$EDITOR`

### publish_post
`publish_post` will:

1. Create a pull request for the current branch, if it doesn't already exist
1. Wait until the status checks pass
1. Render the post in Chrome (assumes that you have a Markdown renderer [like this one](https://chromewebstore.google.com/detail/markdown-viewer/ckkdlimhmcjmikdlpkmbgfkaikojcbjk) installed)
1. Show instructions for creating a new link on Substack
   > 💡 If you provide a `SUBSTACK_URL` value in `.overpowered-writing.env` in the root of your repository, then that value will get used to display the link and the link will be clickable.

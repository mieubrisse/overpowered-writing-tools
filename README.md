Writing Tools
=============
This repository is for people who manage their writing repo in Markdown format.

It contains tools to keep you writing at the speed of thought.

Tools
-----
### jump_post
`jump_post` will jump to a post 


The repo is expected to be in this format:

```
TEMPLATE/
   post.md
   images/
post1/
   post.md
   images/
      some-image.png
      some-other-image.png
post2/
   post.md
   images/
...
```

The tools provided here (contained in `utilities.sh`):

- `jump_post` - Jumps to a post

Usage
-----
First clone this repo somewhere on your machine:
```
git clone git@github.com:mieubrisse/writing-tools.git
```

Then, in your `.bashrc` or `.zshrc`, set the envvar pointing to your writing repo and source `utilities.sh`
```
WRITING_REPO_DIRPATH=/your/writing/repo
source /your/clone/location/writing-tools/utilities.sh
```

3. (Optional, but recommended) Set up aliases to pass

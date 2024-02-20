---
title: Getting Started
section: Basics
---

# Getting Started

## Installation

### Binary

The best way to install `gocden` is to download the latest binary from the [GitHub Releases](https://githubcom/lukeshay/gocden/releases) page.

```bash
curl -o gocden.tar.gz https://github.com/lukeshay/gocden/releases/latest/download/<REPLACE>.tar.gz

tar -xvzf gocden.tar.gz

install ./gocden /usr/local/bin

rm gocden.tar.gz gocden
```

### Homebrew

`gocden` can also be installed using our Homebrew tap:

```bash
brew install lukeshay/tap/gocden

brew install gocden
```

## Setting Up a New Site

You can get started with a new site by running the following command:

```bash
gocden init
```

This command will create a new file called `gocden.toml` in the current directory. This file is used to configure your site.

## Creating a New Page

The default directory for a new site is `docs` but this can be changed in the `gocden.toml` file. To create a page, simply create a new markdown file in the `docs` directory.

```bash
mkdir docs
touch docs/01-index.md
```

### Adding Content

`gocden` requires that all pages have a title. This can be set in the front matter of the markdown file.

`docs/01-index.md`:

```markdown
---
title: My Docs
---

# My Docs

This is where I put my documentation.
```

## Building the Site

Running the following command will walk the source directory, transpile all the markdown files into HTML, and write the output to the output directory. The source and output directories can be configured in `gocden.toml`.

```bash
gocden build
```

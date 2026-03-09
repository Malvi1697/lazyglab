# lazyglab

A terminal UI for GitLab, inspired by [lazygit](https://github.com/jesseduffield/lazygit).

Manage merge requests, pipelines, and issues without leaving your terminal.

## Features

- Browse and switch between GitLab projects
- View, approve, and merge MRs
- Monitor pipelines, view jobs grouped by stage
- Run, retry, and cancel pipelines and individual jobs
- Play manual jobs directly from the TUI
- Filter pipelines by branch
- Browse and manage issues (close/reopen)
- Vim-style keyboard navigation (j/k/h/l/g/G/Ctrl+d/Ctrl+u)
- Context-sensitive keybinding hints at the bottom
- Reads auth from existing `glab` CLI config — zero setup

## Install

### Go install

```bash
go install github.com/Malvi1697/lazyglab@latest
```

### Download binary

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/Malvi1697/lazyglab/releases) page.

### Build from source

```bash
git clone https://github.com/Malvi1697/lazyglab.git
cd lazyglab
make build
```

## Requirements

- [glab](https://gitlab.com/gitlab-org/cli) configured and authenticated with at least one GitLab host

## Usage

```bash
lazyglab
```

Run it inside a git repo with a GitLab remote, or anywhere — lazyglab will list your accessible projects.

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `q` | Quit |
| `?` | Help overlay |
| `1-4` | Switch panel |
| `h/l` | Previous/next panel |
| `Tab/S-Tab` | Next/previous panel |
| `j/k` | Navigate down/up |
| `g/G` | Go to top/bottom |
| `Ctrl+d/u` | Half page down/up |
| `Enter` | Select / view detail |
| `Esc` | Go back / clear filter |
| `b` | Select branch |
| `r` | Refresh |
| `o` | Open in browser |

### Merge Requests

| Key | Action |
|-----|--------|
| `a` | Approve MR |
| `m` | Merge MR |

### Pipelines

| Key | Action |
|-----|--------|
| `Enter` | View jobs |
| `p` | Run new pipeline |
| `R` | Retry pipeline |
| `C` | Cancel pipeline |

### Jobs (inside job view)

| Key | Action |
|-----|--------|
| `R` | Retry job |
| `C` | Cancel job |
| `p` | Play manual job |
| `o` | Open in browser |
| `Esc` | Back to pipeline list |

### Issues

| Key | Action |
|-----|--------|
| `c` | Close/reopen issue |

## Development

```bash
make check         # Run vet + lint + tests
make run           # Build and run
make cover         # Test with coverage report
make fmt           # Format code
make release-dry   # Test GoReleaser locally
```

## Releasing

Push a version tag to trigger a GitHub Actions release with cross-compiled binaries:

```bash
git tag v0.1.0
git push --tags
```

## License

[MIT](LICENSE)

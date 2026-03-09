# lazyglab

A terminal UI for GitLab, inspired by [lazygit](https://github.com/jesseduffield/lazygit).

Manage merge requests, pipelines, and issues without leaving your terminal.

## Features (planned)

- Browse and switch between GitLab projects
- View, approve, and merge MRs
- Monitor pipelines, retry failed jobs, cancel running ones
- Browse and manage issues
- Vim-style keyboard navigation
- Reads auth from existing `glab` CLI config — zero setup

## Requirements

- Go 1.26+
- [glab](https://gitlab.com/gitlab-org/cli) configured with at least one GitLab host

## Install

```bash
go install github.com/Malvi1697/lazyglab@latest
```

Or build from source:

```bash
git clone https://github.com/Malvi1697/lazyglab.git
cd lazyglab
go build -o lazyglab .
```

## Usage

```bash
lazyglab
```

If run inside a git repo with a GitLab remote, lazyglab will auto-detect the project.

## Keybindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `?` | Help |
| `1-4` | Switch panel |
| `Tab` | Next panel |
| `j/k` | Navigate up/down |
| `Enter` | View details |
| `Esc` | Go back |
| `/` | Search/filter |
| `r` | Refresh |

### MR-specific
| Key | Action |
|-----|--------|
| `a` | Approve |
| `m` | Merge |
| `o` | Open in browser |

### Pipeline-specific
| Key | Action |
|-----|--------|
| `R` | Retry pipeline |
| `C` | Cancel pipeline |

## License

MIT

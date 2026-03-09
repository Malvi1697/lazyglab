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
- Interactive first-run setup wizard — no external tools required
- Can import existing `glab` CLI config automatically

## Install

### Quick install (Linux / macOS)

```bash
curl -sL https://raw.githubusercontent.com/Malvi1697/lazyglab/master/install.sh | sh
```

### Go install

```bash
go install github.com/Malvi1697/lazyglab@latest
```

### Download binary

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/Malvi1697/lazyglab/releases) page.

### Debian / Ubuntu (.deb)

```bash
# Download from the latest release
sudo dpkg -i lazyglab_*_linux_amd64.deb
```

### RHEL / Fedora (.rpm)

```bash
# Download from the latest release
sudo rpm -i lazyglab_*_linux_amd64.rpm
```

### Build from source

```bash
git clone https://github.com/Malvi1697/lazyglab.git
cd lazyglab
make build
```

## First Run

On first launch, lazyglab will:

1. Check for an existing `glab` CLI config and offer to import it
2. If no glab config is found, run an interactive setup wizard:

```
$ lazyglab

  No config found. Let's set up lazyglab.

  GitLab host [gitlab.com]:
  Personal access token: ****

  Testing connection... OK (logged in as @you)
  Config saved to ~/.config/lazyglab/config.yml
```

You'll need a [Personal Access Token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) with `api` scope (or `read_api` for read-only).

To reconfigure later: `lazyglab setup`

## Usage

```bash
lazyglab            # Launch the TUI
lazyglab setup      # Re-run setup wizard
lazyglab --version  # Show version
```

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

## License

[MIT](LICENSE)

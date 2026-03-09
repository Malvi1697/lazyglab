# Auto-detect Project + Collapsible Panels

## Git Remote Detection

New `internal/util/gitremote.go` parses git remotes from the current working directory.

- Run `git remote -v` to get remote URLs
- Parse GitLab project paths from both URL formats:
  - `https://gitlab.com/owner/project.git`
  - `git@gitlab.com:owner/project.git`
- Priority: prefer `origin` remote, fall back to other remotes
- Only match remotes whose host matches a configured GitLab host in config

## Auto-select Flow

- After projects load (`ProjectsLoadedMsg`), match detected remote against project list by path (case-insensitive)
- Match found: auto-select project, load MRs/pipelines/issues immediately
- No match: do nothing, user picks manually
- No prompts, no blocking — silent and fast

## Collapsible Sidebar Panels

- Unfocused panel = collapsed to 1 line (title border + single content line)
- Focused panel = expanded, gets remaining vertical space
- Projects panel when collapsed shows: `namespace/my-project → main`
- Projects panel when no selection shows: `No project selected`
- Other panels (MRs, Pipelines, Issues) collapse when unfocused — show selected/highlighted item
- Transition is instant on panel switch (Tab, 1-4, h/l)

## Layout Change

Current: all 4 panels share sidebar space roughly equally.

New: focused panel gets remaining sidebar height after allocating 3 lines per collapsed panel (title border + 1 content line + bottom border). With 3 collapsed panels = 9 lines, the focused panel gets the rest.

# Roadmap

Open work only. What is done is in the README; why it is built the way it is, in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md); how each design came about, in
`docs/specs/` and `docs/plans/`.

## Worth doing next

- **Create a merge request** from the app: source and target branch, title, draft
  flag. The one everyday GitLab action still missing.
- **Create an issue**, same argument.
- **Line-level review comments.** The MR page reads diffs and posts discussion
  comments; a comment anchored to a file and line needs the position payload.
- **Follow a running job's log.** The log is fetched once; a running job would want
  the tail appended while it is open, without re-fetching the whole trace.

## Nice to have

- Default filters, e.g. only MRs assigned to or reviewed by me.
- Custom keybindings from the config file.
- A theme block for the palette, for terminals whose 16 colours are unhelpful.
- Homebrew tap and an AUR package. The updater already declines to overwrite a
  package-manager copy, so both are safe to add.

## Known limitations

- `p` runs a pipeline on the branch head, never on an arbitrary past commit: GitLab
  creates pipelines for a ref, so there is nothing better to do.
- Stage marks need GitLab's GraphQL endpoint. Where it is unavailable the lists draw
  without them.
- One project at a time. Todos is the only view that spans projects, because GitLab's
  to-do endpoint does.

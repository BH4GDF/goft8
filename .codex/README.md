# Codex Project Configuration

This directory contains repository-scoped Codex support files:

- `skills/goft8-improvement/`: project-specific skill for goft8 maintenance work.
- `mcp.toml`: MCP server snippet for local filesystem and Git access.

Some clients only load MCP servers from the user config. In that case, merge
the tables from `mcp.toml` into `~/.codex/config.toml`.

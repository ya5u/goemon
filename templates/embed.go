package templates

import "embed"

//go:embed AGENTS.md
var AgentsMD []byte

//go:embed all:skills
var StandardSkills embed.FS

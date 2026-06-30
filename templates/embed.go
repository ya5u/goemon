package templates

import "embed"

//go:embed AGENTS.md
var AgentsMD []byte

//go:embed skills
var StandardSkills embed.FS

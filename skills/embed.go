// Package skills embeds the agent skill that `pill skill install` writes into
// Pi's skills directory.
//
// //go:embed is a compiler directive: the named file is compiled into the
// binary, so the installed pill needs no files next to it.
package skills

import _ "embed"

// PillSkill is skills/pill/SKILL.md.
//
//go:embed pill/SKILL.md
var PillSkill []byte

package personas

import (
	"embed"
)

//go:embed *.md
var personas embed.FS

func FS() embed.FS {
	return personas
}

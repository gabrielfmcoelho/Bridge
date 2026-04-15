package main

import (
	"embed"

	"github.com/gabrielfmcoelho/ssh-config-manager/cmd"
)

//go:embed all:static
var staticFS embed.FS

func main() {
	cmd.SetStaticFS(staticFS)
	cmd.Execute()
}

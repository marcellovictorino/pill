// notes is a tiny note-taking CLI.
package main

import (
	"os"

	"example.com/notes/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:], os.Stdout, os.Stderr))
}

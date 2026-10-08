// Command pill runs the Pi coding agent against local models served by llama.cpp.
package main

import (
	"os"

	"github.com/marcellovictorino/pill/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}

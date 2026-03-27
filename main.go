// go-solid-score is a static analysis tool that scores Go source code
// against the SOLID design principles.
package main

import (
	"os"

	"github.com/harakeishi/go-solid-score/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}

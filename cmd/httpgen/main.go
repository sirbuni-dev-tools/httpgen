// Command httpgen generates IntelliJ HTTP Client requests and JSON payloads from source code.
package main

import (
	"os"

	"github.com/sirbuni-dev-tools/httpgen/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.StdTerminal()))
}

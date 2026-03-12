// Package main is the entry point for the imagine-tui MCP server.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		fmt.Println("imagine-tui: serve mode not yet implemented")
		return
	}
	fmt.Println("Usage: imagine-tui serve")
}

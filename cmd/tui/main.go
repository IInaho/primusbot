package main

import (
	"context"
	"log"
	"os"

	"nekocode/acp"
	"nekocode/interaction/tui"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--acp" {
		options := acp.ServerOptions{}
		for _, arg := range os.Args[2:] {
			switch arg {
			case "--allow-client-mcp":
				options.AllowClientMCP = true
			default:
				log.Fatalf("unknown ACP option: %s", arg)
			}
		}
		if err := acp.RunStdioWithOptions(context.Background(), options); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := tui.Run(); err != nil {
		log.Fatal(err)
	}
}

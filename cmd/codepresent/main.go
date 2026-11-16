package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cyberbeast/codepresent/pkg/mcpserver"
	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[codepresent] ")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	engine := presentation.NewEngine()
	server := mcpserver.NewServer(engine)

	log.Println("Starting codepresent MCP server on stdio...")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("MCP server exited with error: %v", err)
	}
}

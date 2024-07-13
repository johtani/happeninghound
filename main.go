package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/johtani/happeninghound/client"
)

func main() {
	if err := run(os.Args); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(_ []string) error {
	return client.Run(context.Background())
}

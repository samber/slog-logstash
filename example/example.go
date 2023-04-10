package main

import (
	"fmt"
	"log"
	"time"

	gas "github.com/netbrain/goautosocket"
	sloglogstash "github.com/samber/slog-logtsash"
	"golang.org/x/exp/slog"
)

func main() {
	// ncat -l 9999 -k
	conn, err := gas.Dial("tcp", "localhost:9999")
	if err != nil {
		log.Fatal(err)
	}

	logger := slog.New(sloglogstash.Option{Level: slog.LevelDebug, Conn: conn}.NewLogstashHandler())
	logger = logger.With("release", "v1.0.0")

	logger.
		With(
			slog.Group("user",
				slog.String("id", "user-123"),
				slog.Time("created_at", time.Now().AddDate(0, 0, -1)),
			),
		).
		With("environment", "dev").
		With("error", fmt.Errorf("an error")).
		Error("A message")
}

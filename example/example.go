package main

import (
	"fmt"
	"log"
	"time"

	"log/slog"

	pool "github.com/samber/go-tcp-pool"
	sloglogstash "github.com/samber/slog-logstash/v2"
)

func main() {
	conn, err := pool.Dial("tcp", "localhost:9999")
	if err != nil {
		log.Fatal(err)
	}

	_ = conn.SetPoolSize(10)

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

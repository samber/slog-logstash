
# slog: Logstash handler

[![tag](https://img.shields.io/github/tag/samber/slog-logstash.svg)](https://github.com/samber/slog-logstash/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-%23007d9c)
[![GoDoc](https://godoc.org/github.com/samber/slog-logstash?status.svg)](https://pkg.go.dev/github.com/samber/slog-logstash)
![Build Status](https://github.com/samber/slog-logstash/actions/workflows/test.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/samber/slog-logstash)](https://goreportcard.com/report/github.com/samber/slog-logstash)
[![Coverage](https://img.shields.io/codecov/c/github/samber/slog-logstash)](https://codecov.io/gh/samber/slog-logstash)
[![Contributors](https://img.shields.io/github/contributors/samber/slog-logstash)](https://github.com/samber/slog-logstash/graphs/contributors)
[![License](https://img.shields.io/github/license/samber/slog-logstash)](./LICENSE)

A [Logstash](https://www.elastic.co/logstash/) Handler for [slog](https://pkg.go.dev/log/slog) Go library.

**See also:**

- [slog-multi](https://github.com/samber/slog-multi): `slog.Handler` chaining, fanout, routing, failover, load balancing...
- [slog-formatter](https://github.com/samber/slog-formatter): `slog` attribute formatting
- [slog-sampling](https://github.com/samber/slog-sampling): `slog` sampling policy

**HTTP middlewares:**

- [slog-gin](https://github.com/samber/slog-gin): Gin middleware for `slog` logger
- [slog-echo](https://github.com/samber/slog-echo): Echo middleware for `slog` logger
- [slog-fiber](https://github.com/samber/slog-fiber): Fiber middleware for `slog` logger
- [slog-chi](https://github.com/samber/slog-chi): Chi middleware for `slog` logger
- [slog-http](https://github.com/samber/slog-http): `net/http` middleware for `slog` logger

**Loggers:**

- [slog-zap](https://github.com/samber/slog-zap): A `slog` handler for `Zap`
- [slog-zerolog](https://github.com/samber/slog-zerolog): A `slog` handler for `Zerolog`
- [slog-logrus](https://github.com/samber/slog-logrus): A `slog` handler for `Logrus`

**Log sinks:**

- [slog-datadog](https://github.com/samber/slog-datadog): A `slog` handler for `Datadog`
- [slog-betterstack](https://github.com/samber/slog-betterstack): A `slog` handler for `Betterstack`
- [slog-rollbar](https://github.com/samber/slog-rollbar): A `slog` handler for `Rollbar`
- [slog-loki](https://github.com/samber/slog-loki): A `slog` handler for `Loki`
- [slog-sentry](https://github.com/samber/slog-sentry): A `slog` handler for `Sentry`
- [slog-syslog](https://github.com/samber/slog-syslog): A `slog` handler for `Syslog`
- [slog-logstash](https://github.com/samber/slog-logstash): A `slog` handler for `Logstash`
- [slog-fluentd](https://github.com/samber/slog-fluentd): A `slog` handler for `Fluentd`
- [slog-graylog](https://github.com/samber/slog-graylog): A `slog` handler for `Graylog`
- [slog-slack](https://github.com/samber/slog-slack): A `slog` handler for `Slack`
- [slog-telegram](https://github.com/samber/slog-telegram): A `slog` handler for `Telegram`
- [slog-mattermost](https://github.com/samber/slog-mattermost): A `slog` handler for `Mattermost`
- [slog-microsoft-teams](https://github.com/samber/slog-microsoft-teams): A `slog` handler for `Microsoft Teams`
- [slog-webhook](https://github.com/samber/slog-webhook): A `slog` handler for `Webhook`
- [slog-kafka](https://github.com/samber/slog-kafka): A `slog` handler for `Kafka`
- [slog-nats](https://github.com/samber/slog-nats): A `slog` handler for `NATS`
- [slog-parquet](https://github.com/samber/slog-parquet): A `slog` handler for `Parquet` + `Object Storage`
- [slog-channel](https://github.com/samber/slog-channel): A `slog` handler for Go channels

## 🚀 Install

```sh
go get github.com/samber/slog-logstash/v2
```

**Compatibility**: go >= 1.21

No breaking changes will be made to exported APIs before v3.0.0.

## 💡 Usage

GoDoc: [https://pkg.go.dev/github.com/samber/slog-logstash/v2](https://pkg.go.dev/github.com/samber/slog-logstash/v2)

### Handler options

```go
type Option struct {
	// log level (default: debug)
	Level slog.Leveler

	// connection to logstash
	Conn net.Conn

	// optional: customize json payload builder
	Converter Converter
	// optional: custom marshaler
	Marshaler func(v any) ([]byte, error)
    // optional: fetch attributes from context
    AttrFromContext []func(ctx context.Context) []slog.Attr

	// optional: see slog.HandlerOptions
	AddSource   bool
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}
```

Attributes will be injected in log payload.

Other global parameters:

```go
sloglogstash.SourceKey = "source"
sloglogstash.ContextKey = "extra"
sloglogstash.ErrorKeys = []string{"error", "err"}
```

### Example

```go
import (
    pool "github.com/samber/go-tcp-pool"
    sloglogstash "github.com/samber/slog-logstash/v2"
    "log/slog"
)

func main() {
    // ncat -l 9999 -k
    conn, err := pool.Dial("tcp", "localhost:9999")
    if err != nil {
        log.Fatal(err)
    }

    logger := slog.New(sloglogstash.Option{Level: slog.LevelDebug, Conn: conn}.NewLogstashHandler())
    logger = logger.
        With("environment", "dev").
        With("release", "v1.0.0")

    // log error
    logger.
        With("category", "sql").
        With("query.statement", "SELECT COUNT(*) FROM users;").
        With("query.duration", 1*time.Second).
        With("error", fmt.Errorf("could not count users")).
        Error("caramba!")

    // log user signup
    logger.
        With(
            slog.Group("user",
                slog.String("id", "user-123"),
                slog.Time("created_at", time.Now()),
            ),
        ).
        Info("user registration")
}
```

Output:

```json
{
    "@timestamp":"2023-04-10T14:00:0.000000+00:00",
    "level":"ERROR",
    "message":"caramba!",
    "error":{
        "error":"could not count users",
        "kind":"*errors.errorString",
        "stack":null
    },
    "extra":{
        "environment":"dev",
        "release":"v1.0.0",
        "category":"sql",
        "query.statement":"SELECT COUNT(*) FROM users;",
        "query.duration": "1s"
    }
}


{
    "@timestamp":"2023-04-10T14:00:0.000000+00:00",
    "level":"INFO",
    "message":"user registration",
    "error":null,
    "extra":{
        "environment":"dev",
        "release":"v1.0.0",
        "user":{
            "id":"user-123",
            "created_at":"2023-04-10T14:00:0.000000+00:00"
        }
    }
}
```

### Tracing

Import the samber/slog-otel library.

```go
import (
	sloglogstash "github.com/samber/slog-logstash"
	slogotel "github.com/samber/slog-otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
	)
	tracer := tp.Tracer("hello/world")

	ctx, span := tracer.Start(context.Background(), "foo")
	defer span.End()

	span.AddEvent("bar")

	logger := slog.New(
		sloglogstash.Option{
			// ...
			AttrFromContext: []func(ctx context.Context) []slog.Attr{
				slogotel.ExtractOtelAttrFromContext([]string{"tracing"}, "trace_id", "span_id"),
			},
		}.NewLogstashHandler(),
	)

	logger.ErrorContext(ctx, "a message")
}
```

## 🤝 Contributing

- Ping me on twitter [@samuelberthe](https://twitter.com/samuelberthe) (DMs, mentions, whatever :))
- Fork the [project](https://github.com/samber/slog-logstash)
- Fix [open issues](https://github.com/samber/slog-logstash/issues) or request new features

Don't hesitate ;)

```bash
# Install some dev dependencies
make tools

# Run tests
make test
# or
make watch-test
```

## 👤 Contributors

![Contributors](https://contrib.rocks/image?repo=samber/slog-logstash)

## 💫 Show your support

Give a ⭐️ if this project helped you!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/samber?style=for-the-badge)](https://github.com/sponsors/samber)

## 📝 License

Copyright © 2023 [Samuel Berthe](https://github.com/samber).

This project is [MIT](./LICENSE) licensed.

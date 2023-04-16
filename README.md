
# slog: Logstash handler

[![tag](https://img.shields.io/github/tag/samber/slog-logstash.svg)](https://github.com/samber/slog-logstash/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.20.1-%23007d9c)
[![GoDoc](https://godoc.org/github.com/samber/slog-logstash?status.svg)](https://pkg.go.dev/github.com/samber/slog-logstash)
![Build Status](https://github.com/samber/slog-logstash/actions/workflows/test.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/samber/slog-logstash)](https://goreportcard.com/report/github.com/samber/slog-logstash)
[![Coverage](https://img.shields.io/codecov/c/github/samber/slog-logstash)](https://codecov.io/gh/samber/slog-logstash)
[![Contributors](https://img.shields.io/github/contributors/samber/slog-logstash)](https://github.com/samber/slog-logstash/graphs/contributors)
[![License](https://img.shields.io/github/license/samber/slog-logstash)](./LICENSE)

A [Logstash](https://www.elastic.co/logstash/) Handler for [slog](https://pkg.go.dev/golang.org/x/exp/slog) Go library.

**See also:**

- [slog-multi](https://github.com/samber/slog-multi): workflows of `slog` handlers (pipeline, fanout, ...)
- [slog-formatter](https://github.com/samber/slog-formatter): `slog` attribute formatting
- [slog-gin](https://github.com/samber/slog-gin): Gin middleware for `slog` logger
- [slog-datadog](https://github.com/samber/slog-datadog): A `slog` handler for `Datadog`
- [slog-slack](https://github.com/samber/slog-slack): A `slog` handler for `Slack`
- [slog-loki](https://github.com/samber/slog-loki): A `slog` handler for `Loki`
- [slog-sentry](https://github.com/samber/slog-sentry): A `slog` handler for `Sentry`
- [slog-fluentd](https://github.com/samber/slog-fluentd): A `slog` handler for `Fluentd`
- [slog-syslog](https://github.com/samber/slog-syslog): A `slog` handler for `Syslog`
- [slog-graylog](https://github.com/samber/slog-graylog): A `slog` handler for `Graylog`

## 🚀 Install

```sh
go get github.com/samber/slog-logstash
```

**Compatibility**: go >= 1.20.1

This library is v0 and follows SemVer strictly. On `slog` final release (go 1.21), this library will go v1.

No breaking changes will be made to exported APIs before v1.0.0.

## 💡 Usage

GoDoc: [https://pkg.go.dev/github.com/samber/slog-logstash](https://pkg.go.dev/github.com/samber/slog-logstash)

### Handler options

```go
type Option struct {
	// log level (default: debug)
	Level slog.Leveler

	// connection to logstash
	Conn net.Conn

	// optional: customize json payload builder
	Converter Converter
}
```

Attributes will be injected in log payload.

### Example

```go
import (
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

package sloglogstash

import (
	"context"
	"encoding/json"
	"net"

	"log/slog"

	slogcommon "github.com/samber/slog-common"
)

type Option struct {
	// log level (default: debug)
	Level slog.Leveler

	// connection to logstash
	Conn net.Conn

	// optional: customize json payload builder
	Converter Converter

	// optional: see slog.HandlerOptions
	AddSource   bool
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

func (o Option) NewLogstashHandler() slog.Handler {
	if o.Level == nil {
		o.Level = slog.LevelDebug
	}

	if o.Conn == nil {
		panic("missing logstash connections")
	}

	return &LogstashHandler{
		option: o,
		attrs:  []slog.Attr{},
		groups: []string{},
	}
}

var _ slog.Handler = (*LogstashHandler)(nil)

type LogstashHandler struct {
	option Option
	attrs  []slog.Attr
	groups []string
}

func (h *LogstashHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.option.Level.Level()
}

func (h *LogstashHandler) Handle(ctx context.Context, record slog.Record) error {
	converter := DefaultConverter
	if h.option.Converter != nil {
		converter = h.option.Converter
	}

	message := converter(h.option.AddSource, h.option.ReplaceAttr, h.attrs, h.groups, &record)

	bytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	go func() {
		_, _ = h.option.Conn.Write(append(bytes, byte('\n')))
	}()

	return err
}

func (h *LogstashHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogstashHandler{
		option: h.option,
		attrs:  slogcommon.AppendAttrsToGroup(h.groups, h.attrs, attrs...),
		groups: h.groups,
	}
}

func (h *LogstashHandler) WithGroup(name string) slog.Handler {
	return &LogstashHandler{
		option: h.option,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}

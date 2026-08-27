package log_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	kitlog "github.com/zhuud/boot/log"
)

func Example_dynamicLevel() {
	var output bytes.Buffer
	var level slog.LevelVar
	level.Set(slog.LevelInfo)

	logger := slog.New(kitlog.NewHandler(
		kitlog.WithWriter(&output),
		kitlog.WithLevel(&level),
	))

	logger.Debug("hidden")
	level.Set(slog.LevelDebug)
	logger.Debug("visible")

	fmt.Println(strings.Contains(output.String(), "visible"))
	// Output: true
}

func Example_multiHandler() {
	var textOutput, jsonOutput bytes.Buffer

	logger := slog.New(slog.NewMultiHandler(
		kitlog.NewHandler(kitlog.WithWriter(&textOutput)),
		kitlog.NewHandler(
			kitlog.WithWriter(&jsonOutput),
			kitlog.WithFormat(kitlog.FormatJSON),
		),
	))
	logger.Info("ready")

	fmt.Println(strings.Contains(textOutput.String(), "msg=ready"))
	fmt.Println(strings.Contains(jsonOutput.String(), `"msg":"ready"`))
	// Output:
	// true
	// true
}

func Example_contextAttrs() {
	var output bytes.Buffer
	logger := slog.New(kitlog.NewHandler(
		kitlog.WithWriter(&output),
		kitlog.WithFormat(kitlog.FormatJSON),
	))
	ctx := kitlog.ContextWithAttrs(context.Background(), slog.String("request_id", "req-1"))
	logger.InfoContext(ctx, "handled")

	fmt.Println(strings.Contains(output.String(), `"request_id":"req-1"`))
	// Output: true
}

func Example_redactKey() {
	var output bytes.Buffer
	logger := slog.New(kitlog.NewHandler(
		kitlog.WithWriter(&output),
		kitlog.WithFormat(kitlog.FormatJSON),
		kitlog.WithRedactKey("password"),
	))
	logger.Info("login", slog.String("password", "secret"), slog.String("user_id", "u-1"))

	fmt.Println(strings.Contains(output.String(), `"password":"***"`))
	fmt.Println(strings.Contains(output.String(), "secret"))
	// Output:
	// true
	// false
}

package main

import (
	"context"
	"fmt"
	"io"
	"os"

	fdk "github.com/fnproject/fdk-go"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(handle))
}

func handle(ctx context.Context, in io.Reader, out io.Writer) {
	evt, err := parseEvent(in)
	if err != nil {
		panic(fmt.Errorf("parse event: %w", err))
	}

	connectorID := os.Getenv("SERVICE_CONNECTOR_OCID")
	region := os.Getenv("SERVICE_CONNECTOR_REGION")

	mgr, err := newSCHConnectorManager(region)
	if err != nil {
		panic(fmt.Errorf("connector manager: %w", err))
	}

	res, err := reconcile(ctx, mgr, connectorID, evt)
	if err != nil {
		panic(fmt.Errorf("reconcile: %w", err))
	}

	fmt.Fprintf(out, "action=%s changed=%v before=%d after=%d logGroup=%s\n",
		res.Action, res.Changed, res.Before, res.After, res.LogGroup)
}

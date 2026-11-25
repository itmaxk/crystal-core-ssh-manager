//go:build !windows

package app

import "context"

func startGlobalHotkey(_ context.Context, _ func()) {}

//go:build windows

package main

func defaultSandboxBackend() string {
	if appContainerAvailable() {
		return "appcontainer"
	}
	return "portable-copy"
}

func appContainerAvailable() bool {
	return false
}

func runAppContainerCommand(workspace string, command string, timeoutSec int) (commandResult, error) {
	// The workspace isolation and post-command diff are handled by sandbox.go.
	// A real AppContainer process launcher can replace this single function
	// without changing the connector task protocol or diff pipeline.
	return runCommandInDir(workspace, command, timeoutSec)
}

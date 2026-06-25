package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (client connectorClient) pollLoop() {
	for {
		var envelope taskEnvelope
		err := client.doJSON(http.MethodGet, "/api/advanced-chat/connectors/tasks/next", nil, &envelope)
		if err != nil {
			fmt.Printf("poll failed: %v\n", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if envelope.Task == nil || envelope.Task.ID == "" {
			continue
		}
		fmt.Printf("\nTask %s: %s\n", envelope.Task.ID, envelope.Task.Action)
		result, execErr := client.executeTask(*envelope.Task)
		payload := map[string]interface{}{"success": execErr == nil, "result": result}
		if execErr != nil {
			payload["error"] = execErr.Error()
			fmt.Printf("Task failed: %v\n", execErr)
		} else {
			fmt.Println("Task completed")
		}
		path := "/api/advanced-chat/connectors/tasks/" + envelope.Task.ID + "/result"
		if err := client.doJSON(http.MethodPost, path, payload, nil); err != nil {
			fmt.Printf("result upload failed: %v\n", err)
		}
	}
}

func (client connectorClient) executeTask(task connectorTask) (string, error) {
	workspace, err := client.workspaceRoot(task.WorkspacePath)
	if err != nil {
		return "", err
	}
	if !client.confirm(task) {
		return "", errors.New("denied by local user")
	}
	switch task.Action {
	case "list_files":
		return listFiles(workspace, stringArg(task.Payload, "path"), intArg(task.Payload, "max_entries", 100))
	case "read_file":
		return readFile(workspace, stringArg(task.Payload, "path"), intArg(task.Payload, "max_bytes", 120000))
	case "write_file":
		return writeFile(workspace, stringArg(task.Payload, "path"), stringArg(task.Payload, "content"), boolArg(task.Payload, "overwrite"), boolArg(task.Payload, "create_dirs"))
	case "replace_text":
		return replaceText(workspace, stringArg(task.Payload, "path"), stringArg(task.Payload, "old_text"), stringArg(task.Payload, "new_text"), boolArg(task.Payload, "all"))
	default:
		return "", fmt.Errorf("unsupported action %q", task.Action)
	}
}

func (client connectorClient) confirm(task connectorTask) bool {
	if client.config.AutoApprove || !taskRequiresApproval(task.Action) {
		return true
	}
	fmt.Printf("Workspace: %s\n", task.WorkspacePath)
	if path := stringArg(task.Payload, "path"); path != "" {
		fmt.Printf("Path: %s\n", path)
	}
	switch task.Action {
	case "write_file":
		fmt.Printf("Write bytes: %d\n", len(stringArg(task.Payload, "content")))
	case "replace_text":
		fmt.Printf("Replace bytes: old=%d new=%d\n", len(stringArg(task.Payload, "old_text")), len(stringArg(task.Payload, "new_text")))
	}
	fmt.Print("Approve this connector task? Type yes to allow: ")
	answer, _ := client.stdin.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "yes" || answer == "y"
}

func taskRequiresApproval(action string) bool {
	return action == "write_file" || action == "replace_text"
}

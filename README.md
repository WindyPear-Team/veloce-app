# Veloce App Connector

Local connector for WindyPear agent chat assistant mode.

The connector registers a local device with the backend, polls for approved
tool tasks, and sends task results back to the backend. The workspace directory
is selected in the web session settings.

## Run

Generate a connector token in agent chat > Devices, then run:

```powershell
go run . -server http://localhost:8080 -token <connector-token>
```

If no mode is specified, the connector runs in the standard platform mode for
workspace file and command tools. Website devices run a static site server in
addition to the normal task receiver:

```powershell
go run . -server http://localhost:8080 -token <connector-token> -mode web_server -web-port 8080
```

Use `-data-dir <path>` to choose where hosted sites are stored. By default the
connector uses the user config directory and stores sites under `sites/<domain>`.

## Permissions

Workspace file and command tasks usually run against the absolute workspace path
selected in the web session settings, and the connector verifies that directory
exists locally before those tools run. Paths from the model must be relative to
the workspace root.

Message channels may be configured without a workspace limit. In that mode,
file paths must be absolute paths on the connector device. On Windows, the model
can call `list_windows_drives` to discover available drive roots before choosing
an absolute path.

Agent skills are loaded from both the selected workspace `.agents` directory and
the user-level `~/veloce/.agents` directory. User-level skills are returned
under `.agents/global/...` so they do not collide with workspace-local skills.
Network tasks do not require a workspace path.

Read-only actions run directly:

- `list_files`
- `read_file`
- `list_windows_drives` (Windows connectors only)
- `web_search` (supports `auto`, `duckduckgo`, `bing`, `baidu`, and `google`)
- `web_fetch`

Editing actions require approval in the web frontend before the connector can
receive the task, unless automatic approval is enabled for the chat session:

- `write_file`
- `replace_text`

Command execution always requires approval unless the full command starts with
one of the prefixes allowed in the chat session settings:

- `run_command`

Website devices also accept static site tasks:

- `deploy_static_site`
- `set_static_site_enabled`
- `delete_static_site`

Static site routing uses the HTTP `Host` header. Unknown hosts return 404, and
suspended sites return 403. Deployments write to a temporary directory first and
then atomically swap the site's `public` directory.

## Build

```powershell
go build -o veloce-app.exe .
```
## Special thanks

[Linuxdo](https://linux.do)

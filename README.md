# cnbapplifecycle

Lifecycle that produces Cloud Foundry droplets using Cloud Native Buildpacks.

## Builder

| Flag(s)                   | Type       | Description                                     | Default                 |
| ------------------------- | ---------- | ----------------------------------------------- | ----------------------- |
| `-b`, `--buildpacks`      | `[]string` | buildpacks to use                               |                         |
| `--system-buildpacks-dir` | `string`   | directory where system buildpacks are located   | `/tmp/buildpacks`       |
| `-d`, `--droplet`         | `string`   | output droplet file                             | `/tmp/droplet`          |
| `-r`, `--result`          | `string`   | result file                                     | `/tmp/result.json`      |
| `-w`, `--workspaceDir`    | `string`   | app workspace dir                               | `/home/vcap/workspace`  |
| `-l`, `--layers`          | `string`   | layers dir                                      | `/home/vcap/layers`     |
| `--pass-env-var`          | `[]string` | environment variable(s) to pass to buildpacks   |                         |
| `-c`, `--cache-dir`       | `string`   | cache dir                                       | `/tmp/cache`            |
| `--cache-output`          | `string`   | cache output                                    | `/tmp/cache-output.tgz` |
| `--auto-detect`           | `bool`     | run auto-detection with the provided buildpacks | `false`                 |

### Metadata

Example

```json
{
  "lifecycle_metadata": {
    "buildpacks": [
      {
        "key": "paketo-buildpacks/node-engine",
        "name": "paketo-buildpacks/node-engine@3.2.1",
        "version": "3.2.1"
      },
      {
        "key": "paketo-buildpacks/npm-install",
        "name": "paketo-buildpacks/npm-install@1.5.0",
        "version": "1.5.0"
      },
      {
        "key": "paketo-buildpacks/node-start",
        "name": "paketo-buildpacks/node-start@1.1.5",
        "version": "1.1.5"
      },
      {
        "key": "paketo-buildpacks/npm-start",
        "name": "paketo-buildpacks/npm-start@1.0.17",
        "version": "1.0.17"
      }
    ]
  },
  "process_types": { "web": "sh /home/vcap/workspace/start.sh" },
  "execution_metadata": "",
  "lifecycle_type": "cnb"
}
```

## Launcher

Reads `config/metadata.toml` from `CNB_LAYERS_DIR` (default `/home/vcap/layers`) and launches the application using the Cloud Native Buildpacks [launcher](https://github.com/buildpacks/lifecycle).

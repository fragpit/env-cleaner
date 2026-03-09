# env-cleaner

## Table of Contents

- [Description](#description)
- [How It Works](#how-it-works)
- [Deleting Environments](#deleting-environments)
- [Configuration](#configuration)
- [Environment Metadata](#environment-metadata)
- [Connectors](#connectors)
  - [vSphere](#vsphere)
  - [Helm](#helm)
- [Database](#database)
  - [Database Structure](#database-structure)
- [API](#api)
  - [GET /extend](#get-extend)
  - [GET /extend/apply](#get-extendapply)
  - [GET /api/environments](#get-apienvironments)
  - [POST /api/environments](#post-apienvironments)
- [Notifications](#notifications)
  - [Found Environment Without Metadata](#found-environment-without-metadata)
  - [Environment Is Stale](#environment-is-stale)
  - [Environment Has Been Deleted](#environment-has-been-deleted)
- [Usage](#usage)
  - [Server](#server)
  - [CLI Client](#cli-client)
- [Building](#building)
- [Alternatives](#alternatives)
- [Additional Information](#additional-information)

## Description

A service for monitoring user environments and their automatic deletion.

The service consists of the following parts:

- API for extending environment lifetime.
- Environment remover (Deleter).
- Environment data collector (Crawler).
- Environment metadata storage.

Environment types (what we want to delete):

- Helm release
- Virtual machine in vSphere

Connectors:

- Kubernetes
- vSphere

## How It Works

The Crawler periodically collects environment metadata from configured connectors (Helm, vSphere) and stores it in the database. The Deleter checks for outdated environments and removes them. Before deletion, notifications are sent to environment owners with links to extend the lifetime. The API allows users and CI/CD pipelines to extend environment lifetimes and manage environments.

## Deleting Environments

Deletion of Helm environments is performed via `helm uninstall` (including hooks). Optionally Velero Backup is used to back up the environment before deletion. If your environment uses external storage, you need to manually back it up, for example with Helm uninstall hooks.

In the case of vSphere environments, the virtual machine is powered off, renamed, and moved to the folder specified in the configuration `quarantine_folder_id`.

## Configuration

By default, the service looks for a configuration file in `$HOME/.env-cleaner/env-cleaner.yml`.

The service is configured through a configuration file. An example can be found in `contrib/env-cleaner.yml`.

When starting the service, you need to specify the path to the configuration file using the `--config` flag.

Additionally, configuration through environment variables is supported. To do this, add the prefix `EC_` to the option name from the config file.

For example:

```yaml
notifications:
  email:
    enabled: false
```

When passed through environment variables, it will look like this:

```sh
EC_NOTIFICATIONS_EMAIL_ENABLED=false
```

## Environment Metadata

The following metadata is required for the service to work:

- `EC_OWNER` - environment creator.
- `EC_TTL` - environment lifetime (e.g. `1h`, `1d`, `1w`).

## Connectors

### vSphere

Metadata is stored in the virtual machine notes. Metadata is reset when rolling back a VM snapshot, so VM lifetime management is completely transferred to the service side through the database.

Other ways to store metadata were considered but rejected:

- `tags` - both the tag category and its content are configured in vSphere, i.e. we cannot tag a VM with an arbitrary tag.
- `custom attributes` - attributes appear in the user's entire scope of visibility on all objects of the selected type (Virtual Machine). If an attribute is deleted from at least one object, it is deleted from all objects of this type. Since VMs from different departments can be in the same visibility area, they can delete an attribute from their VMs and it will disappear from ours. Therefore, it is not suitable. It is preserved when rolling back a snapshot.

### Helm

Metadata is stored in additional Helm release variables that can be set when installing a chart.

```sh
helm install \
    --set ec_owner=ivanov \
    --set ec_ttl=1d \
    my-release bitnami/wordpress
```

## Database

SQLite or PostgreSQL is used as the database. If `sqlite.database_folder` is configured, only SQLite will be used regardless of the PostgreSQL settings.

### Database Structure

Table `environments`:

| Column        | Description                                 |
|---------------|---------------------------------------------|
| env_id        | Unique environment identifier               |
| type          | Environment type (helm, vm, namespace)      |
| name          | Environment name (VM name, Helm chart name) |
| namespace     | Namespace for Helm environments             |
| owner         | Environment creator                         |
| delete_at     | Deletion date (`2021-01-01 00:00:00`)       |
| delete_at_sec | Deletion date as Unix timestamp             |

Table `tokens`:

| Column | Description    |
|--------|----------------|
| env_id | Environment ID |
| token  | Unique token   |

## API

### GET /extend

Serves an interactive HTML page where the user can choose an extension period for their environment. The page displays environment info (name, type, owner, scheduled deletion date) and three buttons with period options (min, mid, max). This route does not require basic auth - the token parameter provides security.

Parameters:

- `env_id` - environment ID in the database.
- `token` - one-time token for extending the environment.

### GET /extend/apply

Extends the specified environment for the specified period. Returns a JSON response. Called by the extend UI page via JavaScript.

Parameters:

- `env_id` - environment ID in the database.
- `period` - extension period (e.g. `2d`, `4d`, `1w`). Maximum is specified in the configuration. Exceeding it returns an error.
- `token` - one-time token for extending the environment. A unique token is generated for each stale notification and included in the extend link. After a single use the token is deleted, preventing repeated extensions via the same link.

### GET /api/environments

Returns a list of all environments.

### POST /api/environments

Adds a new environment.

Request body:

```json
{
  "name": "my-release",
  "namespace": "default",
  "owner": "ivanov",
  "type": "helm",
  "ttl": "1d"
}
```

## Notifications

### Found Environment Without Metadata

```txt
Environment: vm-name, type: vsphere_vm, is orphaned
```

You need to move the environment to the blacklist, add metadata manually, or move it to a separate folder in vSphere.

### Environment Is Stale

```txt
Environment release-name (namespace: release-ns), type: helm,
is stale and will be deleted in <stale_threshold>

[Extend your environment]
```

The environment has less than `stale_threshold` left until deletion. A notification is sent to the user with a link to the extend UI page. On the page, the user can choose one of three extension periods: `min` equals `stale_threshold`, `max` equals `max_extend_duration`, and `mid` is half of `max`.

### Environment Has Been Deleted

```txt
Environment: release-name (namespace: release-ns),
type: helm, is outdated and has been deleted
```

The environment has been deleted. The user receives a notification about the deletion.

## Usage

### Server

Start the server mode which runs the API, crawler, and deleter:

```sh
env-cleaner server --config /path/to/env-cleaner.yml
```

The server reads its configuration from `$HOME/.env-cleaner/env-cleaner.yml` by default. Use the `--config` flag to specify a custom path. Debug mode can be enabled with the `--debug` (`-d`) flag.

### CLI Client

The CLI client communicates with the server API. It uses a separate configuration file at `$HOME/.env-cleaner/env-cleaner-client.yml` with the following options:

```yaml
api_url: "http://localhost:8080"
admin_api_key: "your-api-key"
```

Available commands:

```sh
env-cleaner environment list               # List all environments
env-cleaner env ls                         # Same using aliases

env-cleaner environment add \              # Add a new environment
    --name my-release \
    --owner ivanov \
    --type helm \
    --ttl 1d \
    --namespace default

env-cleaner version                        # Show version
```

The `add` command requires `--name`, `--owner`, `--type`, and `--ttl` flags. The `--namespace` flag is required when `--type` is `helm`.

## Building

The project uses [Task](https://taskfile.dev/) as a build tool. Available tasks:

```sh
task build        # Build binaries
task lint         # Lint code
task test         # Run tests
task image-build  # Build container images
task image-push   # Push images to registry
```

## Alternatives

[kube-janitor](https://codeberg.org/hjacobs/kube-janitor) - deletes Kubernetes objects based on annotations. It does not suit our needs because we need to delete via `helm uninstall` to activate hooks, and it does not delete VMs in vSphere.

## Additional Information

- [vSphere Web Services API](https://developer.broadcom.com/xapis/vsphere-web-services-api/7.0/)
- [Helm Library](https://github.com/helm/helm)

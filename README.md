# Table of contents

[TOC]

## Description

A service for monitoring user environments and their automatic deletion.

The service consists of the following parts:

* API for extending environment lifetime.
* Environment remover (Deleter)
* Environment data collector (Crawler)
* Environment metadata storage.

Environment types (what we want to delete):

* helm release
* virtual machine in vsphere

Connectors:

* k8s
* vSphere

Environment metadata:

* environment creator
* environment lifetime

## How it works

## Deleting environments

Deletion of helm environments is performed via helm uninstall (including hooks). Optionally Velero Backup is used to backup the environment before deletion. If your environment use external storage, you need to manually backup it, for example with helm uninstall hooks.

In the case of vSphere environments, the virtual machine is powered off, renamed, and
moved to the folder specified in the configuration `quarantine_folder_id`.

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

when passed through environment variables it will look like this:

```sh
EC_NOTIFICATIONS_EMAIL_ENABLED=false
```

## Alternatives

https://codeberg.org/hjacobs/kube-janitor

The service deletes k8s objects based on annotations.
It doesn't suit our needs because we need to delete via helm uninstall to activate hooks,
and it doesn't delete VMs in vSphere.

## Environment metadata

The following metadata is required for the service to work:

* `EC_OWNER` - environment creator.
* `EC_TTL` - environment lifetime. (e.g.: 1h, 1d, 1w)

## Connectors

### VSphere

Metadata is stored in the virtual machine notes. Metadata is reset when
rolling back a VM snapshot, so VM lifetime management is completely transferred
to the service side through the database.

There are other ways to store metadata.

* `tags` - both the tag category and its content are configured in vSphere. i.e.
    we cannot tag a VM with an arbitrary tag. IT seems to have some kind of
    automatic tag generator for the VM_CREATOR category that synchronizes
    with domain users.
* `custom attributes` - attributes appear in the user's entire scope of visibility
    on all objects of the selected type (Virtual Machine). And if an attribute is deleted
    from at least one object, it is deleted from all objects of this type. Since
    VMs from different departments can be in the same visibility area, they can
    delete an attribute from their VMs and it will disappear from ours. Therefore, it's not suitable.
    It is preserved when rolling back a snapshot.

### Helm

Metadata is stored in additional helm release variables that can be
set when installing a chart.

```sh
helm install \
    --set ec_owner=ivanov \
    --set ec_ttl=1d \
    my-release bitnami/wordpress
```

## Database

SQLite or PostgreSQL is used as the database. If `sqlite.database_folder` is configured,
only SQLite will be used regardless of the PostgreSQL settings.

### Database structure

Table environments:

* env_id - unique environment identifier
* type - environment type (helm, vm, namespace)
* name - environment name (VM name, helm chart name)
* namespace - namespace for helm environments.
* owner - environment creator
* delete_at - environment deletion date in format 2021-01-01 00:00:00
* delete_at_dec - environment deletion date in unix timestamp format.

Table tokens:

* env_id - environment id
* token - unique token

## API

### /extend?env_id={env_id}&period={period}&token={token}

Extends the specified environment for the specified period.

* env_id - environment id in the database
* period - environment extension period. 2d, 4d, 1w
* token - one-time token for extending the environment.

period - maximum specified in the configuration. When changing the link and extending for a
longer period, an error is displayed.

### /environment/

## Notification of the user and system administrator

### Found environment without metadata

```txt
**Environment: vm-name, type: vsphere_vm, is orphaned**
```

You need to move the environment to the blacklist, add metadata manually, or move it to a separate folder in vsphere.

### Environment is stale and will be deleted

```txt
**Environment release-name (namespace: release-ns), type: helm, is stale and will be deleted in 3d**
Use one of the following links to extend your environment:
- [Extend 3d]
- [Extend 1w]
- [Extend 2w]
```

The environment has less than `stale_threshold` left until deletion. A notification is sent to the user with a suggestion to extend the environment.

### Environment has been deleted

```txt
**Environment: release-name (namespace: release-ns), type: helm, is outdated and has been deleted**
```

The environment has been deleted. The user receives a notification about the environment deletion.

## Additional information

vSphere API: https://developer.broadcom.com/xapis/vsphere-web-services-api/7.0/

Helm Library: https://github.com/helm/helm

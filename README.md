# dcrcli

![Release](https://img.shields.io/github/v/release/mongodb-labs/dcrcli?label=release)

## Description

dcrcli is a **read-only** command-line utility that copies diagnostic information from a MongoDB deployment so MongoDB Support can review it. It does not change your data, cluster configuration, or running processes.

By default it collects from **one secondary only** (to avoid load on primaries) and gathers the artifacts listed below. You can narrow nodes and/or artifact types interactively or with flags (see [Collection scope](#collection-scope-which-nodes) and [Collection data](#collection-data-which-artifacts)).

## Collection Details (read-only)

dcrcli is a diagnostic collector. It **reads** cluster metadata and **copies** existing files off the host. It does **not** write to your databases.

**What it collects** — a short list you can share with change-control:


| Artifact                                 | What it is                                                             | How it is collected                                                                                    |
| ---------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| **getMongoData**                         | Snapshot of server, replica-set, database, collection, and index stats | Read-only `mongosh` / `mongo` commands                                                                 |
| **FTDC**                                 | MongoDB diagnostic metrics files (`metrics.`*)                         | Local copy or `rsync` over SSH (read of existing files)                                                |
| **Mongod logs**                          | Existing `mongod` / `mongos` log files                                 | Local copy or `rsync` over SSH (read of existing files)                                                |
| `df -h`                                  | Host filesystem usage                                                  | `df` on the node (SSH when the node is remote)                                                         |
| `df -h <dbpath>`                         | Disk usage of the MongoDB data directory                               | Same `df`, only on `mongod` (skipped on mongos)                                                        |
| `ulimit -a`                              | Process resource limits on the MongoDB host                            | Same host as `df`: on that node if dcrcli runs there, otherwise SSH                                    |
| `rs.conf()`                              | Replica set configuration                                              | Read-only shell helper (`mongod`)                                                                      |
| `rs.status()`                            | Replica set member health and lag                                      | Read-only shell helper (`mongod`); also present inside getMongoData, written here as a standalone file |
| `rs.printReplicationInfo()`              | Oplog window                                                           | Read-only shell helper (`mongod`)                                                                      |
| `rs.printSecondaryReplicationInfo()`     | Replication lag                                                        | Read-only shell helper (`mongod`)                                                                      |
| `sh.status()`                            | Sharded-cluster status                                                 | Read-only shell helper (**mongos only**)                                                               |
| `$listCatalog` `system.buckets.*`        | Time-series / bucket collections (including those in user databases)   | Read-only collectionless `$listCatalog` on **admin** on **data-bearing** `mongod` (not mongos, not config-server members) |
| Unique indexes `formatVersion`           | Whether non-`_id` unique indexes are still pre-4.2 / legacy (`13`/`14` = new) | Read-only `$collStats` on **data-bearing** `mongod`, only for collections that have a non-`_id` unique index (same skip as `$listCatalog`; not `validate()`) |
| `serverStatus().shardedIndexConsistency` | Index consistency across shards                                        | Read-only on the **config-server primary** (not mongos)                                                |


The command outputs (`df`, `ulimit`, `rs.*`, `sh.status` on mongos, `$listCatalog` and unique-index `formatVersion` on data-bearing mongods, and `shardedIndexConsistency` on the config primary) are **always** collected for each target node they apply to. Unique-index `formatVersion` runs `$collStats` only on collections that have a non-`_id` unique index. getMongoData, FTDC, and logs can be limited with `-collect-data` (see [Collection data](#collection-data-which-artifacts)).

**Production impact:** the default scope is **one secondary**. Collection is sequential (one node at a time). If any discovered cluster member is unreachable, dcrcli **stops** instead of adding load to a degraded cluster.

You can inspect every file under `./outputs/` locally before attaching anything to a support case.

## Table of Contents

- [Collection Details (read-only)](#collection-details-read-only)
- [Releases](#releases)
- [Prerequisites](#prerequisites)
- [Usage](#usage)
  - [Config File (recommended)](#config-file-recommended)
  - [Collection scope (which nodes)](#collection-scope-which-nodes)
  - [Collection data (which artifacts)](#collection-data-which-artifacts)
  - [Cluster health pre-check](#cluster-health-pre-check)
- [Output Location](#output-location)
- [Internal Notes](#internal-notes)
- [Build from Source](#build-from-source)
- [License](#license)
- [Disclaimer](#disclaimer)
- [Contributing](#contributing)
- [Security](#security)
- [Feedback / Issues](#feedback--issues)



## Releases

Download the latest prebuilt binaries:

- [https://github.com/mongodb-labs/dcrcli/releases](https://github.com/mongodb-labs/dcrcli/releases)



## Prerequisites

For a successful collection, install and check these **before** running dcrcli.

**On the dcrcli host** (the machine where you run `./dcrcli`):


| Need                                                                                       | When                                                                                           |
| ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| **mongosh** (preferred) or **mongo** on `PATH`                                             | Always                                                                                         |
| **rsync** on `PATH`                                                                        | Collecting **FTDC** or **mongod logs** from **remote** nodes (this is the default `all`)       |
| Hostnames of every cluster member resolvable (same names as `rs.status()` / `getShardMap`) | Always                                                                                         |
| TCP to every discovered MongoDB port for the whole run                                     | Always (dcrcli **aborts** if any member is unreachable)                                        |
| SSH from this machine to each remote MongoDB host                                          | Collecting FTDC or logs from nodes that are not local; remote `df` / `ulimit` use the same SSH |
| Free disk on this filesystem (abort below ~**1.1 GB**; plan `(400 × N) + 1024` **MB**)     | Always                                                                                         |


**On each MongoDB host** (when that node is remote and you collect FTDC or logs):


| Need                                             | When                                                                          |
| ------------------------------------------------ | ----------------------------------------------------------------------------- |
| SSH server reachable from the dcrcli host        | FTDC or logs from that host                                                   |
| SSH user can **read** MongoDB FTDC and log files | FTDC or logs from that host                                                   |
| `df` on `PATH`                                   | Remote `df -h` / `df -h <dbpath>`                                             |
| `ulimit`                                         | Shell builtin on that host (not on the dcrcli laptop unless it *is* the node) |


**MongoDB access:**


| Need                                                                                                          | When                                             |
| ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------ |
| Database user with `backup`, `readAnyDatabase`, and `clusterMonitor`                                          | Authentication enabled                           |
| The **same** user as a **shard-local** user on **each shard replica set** (create once on each shard primary) | Sharded cluster, collecting from shard `mongod`s |
| Same shard-local user plus **`directShardOperations`** on **each shard primary** (not only on mongos)         | MongoDB **8.0+** sharded cluster, if you need **getMongoData on each shard `mongod`** |


Details:

1. Network Access

- Hostnames of all nodes in the MongoDB cluster must be resolvable from the machine running dcrcli.
- Use the same hostnames as the cluster configuration (e.g., those shown by rs.status()).
- Allow firewall access from the dcrcli host to MongoDB ports (e.g., 27017, 27018, etc.).
- **Every** discovered node (all `mongod`s, plus `mongos` and config-server members on sharded topologies) must be reachable from the dcrcli host on its listening port for the **whole duration** of the run. dcrcli probes every member before starting and again before each per-node collection step, and **aborts with exit code 1** if any node is unreachable (see [Cluster health pre-check](#cluster-health-pre-check)).
- When **FTDC** or **mongod logs** are collected from nodes that are not on the dcrcli host, allow SSH from the dcrcli host to those nodes. Remote `df -h` and `ulimit -a` use that same SSH access; dcrcli does **not** prompt for SSH only to collect those host commands.

1. MongoDB Shell

- Either the **mongo** or **mongosh** shell must be installed on the machine running dcrcli (dcrcli prefers **mongosh** when both are on `PATH`).
- **Use the latest mongosh** (current stable). This is **strongly recommended**, especially for **sharded clusters** and whenever dcrcli must discover node roles (primary vs secondary). Newer mongosh emits reliable JSON for topology and role checks; the legacy **mongo** shell may not parse the same way, which can leave roles unknown and cause secondary-only collection to fail until you use **mongosh** or choose **all-nodes**.
- Quick checks:

```
which mongosh
```

or

```
echo "$PATH"
```

- If authentication is enabled:
  - Use a database user with the appropriate permissions (see “Minimum Required Permissions” in the getMongoData README: [https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md#more-details](https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md#more-details)). The interactive prompt asks for the `backup`, `readAnyDatabase`, and `clusterMonitor` roles.
  - If the password contains special characters (e.g., $, /, ?, #), input them directly without percent encoding.
  - **Sharded clusters (self-managed / SCRAM):** dcrcli authenticates **directly** to each target `mongod`/`mongos` with the same username and password. A user created **only through mongos** lives on the **config servers**. That user can collect from mongos and CSRS members, but **shard** `mongod`**s will return** `Authentication failed` unless the same user (same password and roles) also exists as a **shard-local** user on **each shard replica set**. Create it once on each **shard primary** (it replicates to that shard’s secondaries). See [Users in Self-Managed Deployments](https://www.mongodb.com/docs/manual/core/security-users/#shard-local-users) (shard-local vs cluster users). Scope that includes shard members (`all-nodes`, `all-secondaries`, or `one-secondary` when the chosen secondary is a shard member) needs those shard-local users. LDAP/x.509 cluster-wide identities are a different setup.
  - **MongoDB 8.0+ sharded clusters:** starting in 8.0, a shard `mongod` only accepts a [limited set of direct commands](https://www.mongodb.com/docs/manual/reference/supported-shard-direct-commands/). Clients should use **mongos**. getMongoData on a shard runs `listCollections` / `getIndexes` / `collStats` on every **local** database; those commands are not on that list. Without [`directShardOperations`](https://www.mongodb.com/docs/manual/reference/built-in-roles/#mongodb-authrole-directShardOperations) on the **shard-local** user, getMongoData can fail on shard `mongod`s that locally have user databases (`You are connecting to a sharded cluster improperly by connecting directly to a shard`). Grant the role on **each shard primary** — granting it **only through mongos** is not enough (that user lives on the config servers; dcrcli authenticates to each shard with the shard-local user). mongos and config-server getMongoData, plus FTDC, logs, and `rs.*` on the shards, still succeed. `directShardOperations` is a **maintenance** role: use it for the collection window, then remove it from the shard-local users. Keep `backup` / `readAnyDatabase` / `clusterMonitor`. Replica sets that are **not** sharded are unaffected. Public docs allow a direct-to-shard exception during **replica set → 1-shard conversion**; that exception **ends once a second shard is added**. A cluster that was **always** 1-shard is **not** documented as exempt.

1. Remote FTDC, logs, `df`, and `ulimit` (SSH / rsync)

- Install **rsync** on the **dcrcli host** (`which rsync`). It is required to copy FTDC and mongod logs from remote nodes. The default collection (`all`) includes those artifacts.
- Required only when collecting **FTDC** or **mongod logs** from remote nodes (`-collect-data` includes `ftdc` or `logs`, which is the default `all`). Using [passwordless SSH](https://linodelinux.com/how-to-setup-ssh-login-without-password-in-linux/) is recommended for an unattended run.
  - Note: rsync over SSH copies FTDC and log files from the hosts to the dcrcli host. If passwordless SSH is not configured, you type the SSH password **once per node** (FTDC, logs, and host commands share one connection). Passwordless SSH is still recommended for unattended runs.
- The SSH user must have read permissions on MongoDB log and FTDC files.
- `df` must exist on each **MongoDB** host (`PATH` on that machine). `ulimit` is a shell builtin on that same host (not on the laptop you run dcrcli from, unless that laptop *is* the node).
- If SSH daemons on nodes use non-default ports, specify them via SSH config on the dcrcli host.
- If hostnames used on MongoDB nodes are not resolvable, add their IP addresses to /etc/hosts on the dcrcli host.

1. Disk space on the dcrcli host

- Outputs are written under `./outputs/<cluster_name>/` (plus `./outputs/temp/` while a remote node is copied). There is no second cluster-level archive during the run; zip/tar the output directory yourself when you are done.
- **Planning estimate:** leave at least `(400 × N) + 1024` **MB** free on the filesystem where you run dcrcli, where `N` **is the number of nodes you will collect from** (same count as collection scope — one secondary → `N = 1`; all-nodes on a 3-member replica set → `N = 3`). Examples: one secondary → about **1.5 GB**; three nodes → about **2.2 GB**.
- This is a **rule of thumb**, not a limit. FTDC is typically ~100–500 MB per `mongod`. **Mongod logs are uncapped** and often dominate; busy hosts with large log files can need several extra GB. Command outputs (`df`, `ulimit`, `rs.`*, `sh.status`) are tiny.
- **During the run**, dcrcli **aborts** if free space on the current working directory’s filesystem drops below about **1.1 GB**. Check before you start:

```
df -h .
```

- After you have copied or compressed what you need to send, you can remove `./outputs/<cluster_name>/` to reclaim space.



## Usage

Follow these steps:

1. Download the latest release: [https://github.com/mongodb-labs/dcrcli/releases](https://github.com/mongodb-labs/dcrcli/releases)
2. Transfer the binary to a machine that can access the MongoDB nodes.
3. Make it executable:

```
chmod +x <binary-name>
```

1. Run using a config file (recommended) or interactively:

**With a config file** — connection details come from the file (easy to re-run and fix). If `username` is set, dcrcli still prompts for the MongoDB password (it is never stored in the file):

```
./<binary-name> -config dcrcli.config.json
```

**Interactively** — follow on-screen prompts for cluster name, seed host/port, MongoDB credentials, which artifacts to collect, SSH username (only if FTDC or logs are selected), and which nodes to collect:

```
./<binary-name>
```

Run `./<binary-name> -h` for a full summary of flags.

Flags:


| Flag                    | Purpose                                                                                                                                                                              |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `-config path`          | Load connection details from a JSON config file (recommended).                                                                                                                       |
| `-generate-config path` | Write a sample config file to `path` and exit.                                                                                                                                       |
| `-collect-nodes mode`   | Collection scope: `one-secondary`, `all-secondaries`, or `all-nodes`.                                                                                                                |
| `-collect-data types`   | Which optional artifacts to collect: `all`, or a comma-separated list of `getmongodata`, `ftdc`, `logs`. Command outputs (`df`, `rs.*`, `sh.status` on mongos) are always collected. |




### Config File (recommended)

A config file lets you set all connection details upfront so you never have to re-enter them. If a run fails, the error message tells you exactly which field to fix — just update the file and re-run.

**Step 1 — Generate a sample file:**

```
./<binary-name> -generate-config dcrcli.config.json
```

This writes a `dcrcli.config.json` file with placeholder values and prints a description of each field. The file is created with `0600` permissions to restrict read access to the config file.

**Step 2 — Edit the file with your values:**

```json
{
  "cluster_name":  "my-cluster",
  "seed_host":     "localhost",
  "seed_port":     "27017",
  "username":      "",
  "uri_options":   "",
  "ssh_username":  "",
  "collect_nodes": "one-secondary",
  "collect_data":  "all"
}
```


| Field           | Description                                                                                                                                                                                                                          |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `cluster_name`  | Display name used for the output directory.                                                                                                                                                                                          |
| `seed_host`     | Reachable mongod/mongos used to discover other cluster members. Defaults to `localhost` if blank.                                                                                                                                    |
| `seed_port`     | Port of the seed node. Defaults to `27017` if blank.                                                                                                                                                                                 |
| `username`      | MongoDB admin username. Leave blank for clusters without authentication. If set, dcrcli prompts for a password at startup (password is never stored in the config file).                                                             |
| `uri_options`   | Extra URI connection options in `name=value&name2=value2` format. **Do not include** `replicaSet` **here** — dcrcli discovers topology itself.                                                                                       |
| `ssh_username`  | OS username for SSH/rsync to remote nodes when collecting FTDC or logs. Leave blank if all nodes are on the same machine as dcrcli. Ignored when `collect_data` is `getmongodata` only (FTDC/logs off; remote `df` is then skipped). |
| `collect_nodes` | Which nodes to collect from: `one-secondary` (default), `all-secondaries`, or `all-nodes`. Leave blank to be prompted interactively.                                                                                                 |
| `collect_data`  | Which optional artifacts to collect: `all` (default), or a comma-separated list of `getmongodata`, `ftdc`, `logs`. Leave blank to be prompted interactively. Command outputs are always collected.                                   |


**Step 3 — Run:**

```
./<binary-name> -config dcrcli.config.json
```

dcrcli prints a summary of what was loaded from the file before proceeding, so you can confirm the values at a glance:

```
Loading config from: dcrcli.config.json
  cluster_name:  my-cluster
  seed_host:     mongo-node1.internal
  seed_port:     27017
  username:      diag_user
  password:      [will prompt interactively]
  uri_options:   (none)
  ssh_username:  ubuntu
  collect_nodes: one-secondary
  collect_data:  all

Enter MongoDB Password:
```

The password prompt does not echo input to the screen. It is never stored in the config file or on disk — it only exists in memory for the duration of the run. For no-auth clusters (username left blank), the prompt is skipped entirely.

If a field fails validation, the error names the field and tells you which file to edit:

```
Config validation failed: config field "uri_options": FATAL: do not enter replicaSet in options
Fix the value in dcrcli.config.json and re-run.
```

> **Note:** The `-collect-nodes` / `-collect-data` flags always take precedence over the matching config file values, which in turn take precedence over the interactive prompts.



### Collection scope (which nodes)

After topology is discovered, dcrcli asks **which nodes to collect from** (unless you pass a flag). You can also pass:

```
./<binary-name> -collect-nodes=one-secondary
./<binary-name> -collect-nodes=all-secondaries
./<binary-name> -collect-nodes=all-nodes
```

Run `./<binary-name> -h` for a short summary of flags.

- If `-collect-nodes` is set, it **overrides** the interactive menu (useful for scripts and CI).
- If stdin is **not** a terminal (non-interactive), the default is `one-secondary` without prompting.


| Value               | Behavior                                                                                                                                                                                                                                                                                                   |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **one-secondary**   | A **single** secondary member only (smallest footprint; no extra mongos/config added).                                                                                                                                                                                                                     |
| **all-secondaries** | **Every** secondary (including config-server members that are secondaries). On a **sharded** topology, dcrcli also adds **one** mongos and **one** config-server `mongod` from `getShardMap` that are not already in that list (first of each when sorted by hostname/port).                               |
| **all-nodes**       | **Every** host dcrcli discovered: all shard `mongod`s (primaries and secondaries), **all** mongos, **all** config-server members. A full cluster capture: more nodes than secondary-only, so the run takes longer and the output directory is larger. Collection is still sequential (one node at a time). |


**Sharded clusters:** Use a **mongos** as the seed host when possible (same as before). For **all-secondaries**, one router and one CSRS member are included when the topology is detected as sharded. `getShardMap` does not always list every mongos; the **seed mongos** is added to the list when missing (and may be the mongos chosen for option 2). `$listCatalog` for `system.buckets.`* and unique-index `formatVersion` run on shard `mongod`s (not mongos). `serverStatus().shardedIndexConsistency` runs on the **config-server primary** only — use **all-nodes** to include that member. If auth is enabled, also create the collection user on **each shard replica set** (see [Prerequisites](#prerequisites)); a mongos-only cluster user is not enough for direct connections to shard `mongod`s. On **MongoDB 8.0+**, getMongoData on shard `mongod`s also needs **`directShardOperations` on each shard-local user** (not only on mongos). Without it, that node’s getMongoData can fail while FTDC/logs/`rs.*` still collect. See [Prerequisites](#prerequisites) for the replica-set → 1-shard conversion exception.

**Replica sets (non-sharded):** **all-secondaries** and **one-secondary** only collect secondary `mongod` members; there is no separate mongos/config layer.

**Standalone (single** `mongod`**):** If only **one** data node is discovered and it is **not** a secondary (normal for standalone), and you use options **1** or **2** without `-collect-nodes`, dcrcli prints a **WARNING** and asks whether to collect from that **primary** anyway (**y** / **yes** to continue). There is no extra prompt when you pass `-collect-nodes` or when stdin is not a terminal—use `-collect-nodes=all-nodes` for unattended standalone runs.

### Collection data (which artifacts)

dcrcli can optionally limit **getMongoData**, **FTDC**, and **mongod logs**. The DCR command outputs (`df -h`, `df -h <dbpath>`, `ulimit -a`, `rs.conf()`, `rs.status()`, `rs.printReplicationInfo()`, `rs.printSecondaryReplicationInfo()`, `$listCatalog` for `system.buckets.`* and unique-index `formatVersion` on data-bearing mongods, `sh.status()` on mongos, and `serverStatus().shardedIndexConsistency` on the config-server primary) are **always** collected for each target node they apply to — they are small, read-only, and not a `-collect-data` type.

By default it collects **all** optional types plus the command outputs. To collect only specific optional types (for example getMongoData), use:

```
./<binary-name> -collect-data=getmongodata
./<binary-name> -collect-data=ftdc
./<binary-name> -collect-data=logs
./<binary-name> -collect-data=getmongodata,ftdc
./<binary-name> -collect-data=all
```

Or set `"collect_data": "getmongodata"` in the config file.

- If `-collect-data` is set, it **overrides** the interactive menu.
- If stdin is **not** a terminal (non-interactive), the default is `all` without prompting.
- When only **getMongoData** is selected (no FTDC or logs), dcrcli skips the SSH username prompt and ignores `ssh_username` in the config file. Remote `df -h` is then skipped for nodes that are not local; replica-set / mongos helpers still run over the MongoDB port.


| Value            | Behavior                                                                                                        |
| ---------------- | --------------------------------------------------------------------------------------------------------------- |
| **all**          | Collect getMongoData, FTDC, and mongod logs (default). Command outputs are still collected.                     |
| **getmongodata** | Run getMongoData / mongoWellnessChecker only (no FTDC or mongod log copy). Command outputs are still collected. |
| **ftdc**         | Copy FTDC metrics only. Command outputs are still collected.                                                    |
| **logs**         | Copy mongod logs only. Command outputs are still collected.                                                     |


Combine types with commas. Aliases: `gmd` / `get-mongo-data` for getMongoData; `mongod-logs` / `log` for logs.

**Interactive menu:** When prompted, choose one of four options:


| Choice          | Collects                                                   |
| --------------- | ---------------------------------------------------------- |
| **1** (default) | getMongoData, FTDC, and mongod logs (plus command outputs) |
| **2**           | getMongoData only (plus command outputs)                   |
| **3**           | FTDC only (plus command outputs)                           |
| **4**           | mongod logs only (plus command outputs)                    |


To combine types (for example getMongoData and logs without FTDC), use `-collect-data` or `collect_data` in the config file — there is no custom free-text option in the interactive menu.

**Collection progress:** During data collection, dcrcli prints a progress bar and a per-node summary when collection finishes. The summary lists **only artifact types that were collected, failed, or selected but could not run** — types omitted from the selection are hidden. Successful tasks show `✓`; failed tasks show `!` (and the node line is marked `!` as well); selected artifacts that could not run (for example FTDC on a remote node with no SSH user) show `−`. Example when only getMongoData was selected:

```
  ✓ mongo1:27017  ✓ getMongoData  ✓ commands
```

Example when all types were selected but FTDC failed on one node:

```
  ! mongo1:27017  ✓ getMongoData  ! FTDC  ✓ logs  ✓ commands
```



### Cluster health pre-check

dcrcli collects diagnostic data (getMongoData, FTDC, and/or mongod logs) against live (typically production) clusters, so it refuses to collect from any node while another cluster member is unreachable. Proceeding in that state can mask a partial outage and adds avoidable load to a cluster that is already degraded. This gate applies for every `-collect-data` selection, not only getMongoData.

The health check is a lightweight TCP probe (5-second timeout per node, sequential) against **every** node discovered by the topology finder — not just the nodes selected by `-collect-nodes`. On a sharded topology this includes all `mongod`s plus the `mongos` and config-server members that were discovered.

It runs in two phases:


| Phase              | When                                                                      | What happens on failure                                                                                                   |
| ------------------ | ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| **pre-collection** | Once, right after topology discovery and before the first node is touched | dcrcli aborts before any `getMongoData`, FTDC, or log copy work runs                                                      |
| **pre-iteration**  | At the start of every per-target iteration of the collection loop         | dcrcli aborts before moving on to the next target, so a mid-run degradation does not stack additional load on the cluster |


When a node is unreachable, dcrcli prints an `ERROR` banner listing every offending `host:port`, records a terminating message in the dcrcli log (`dcrlogfile_*.log`), and exits with **code 1**. Example console output:

```
######################################################################
#                                 ERROR                              #
######################################################################

Cluster health check failed (pre-iteration).
The following MongoDB node(s) are unreachable:
  - shard0-rs1.example.net:27017

dcrcli collects diagnostic data against live clusters; refusing to proceed while any cluster node is down to avoid added production risk.
Verify all members are healthy (e.g. rs.status()) and retry.
```

If you see this, verify the named member with `rs.status()` (or `sh.status()` on a sharded cluster), bring it back, and retry. There is no flag to bypass the check — it is intentional.

## Output Location

- Collected artifacts are written under `./outputs/<cluster_name>/`, with one subdirectory per node (`<hostname>_<port>`). Review this directory locally before sharing it.
- Per node you will typically see:
  - `getMongoData.json` — when getMongoData was selected
  - `ftdcarchive.tar.gz` — when FTDC was selected
  - `logarchive.tar.gz` — when logs were selected
  - DCR command outputs (always collected):
    - `df-h.txt` and `df-h-dbpath.txt` — host disk usage. Remote `df` runs only when SSH was already enabled for FTDC/logs; otherwise the files record that df was skipped. `df -h <dbpath>` is omitted on mongos.
    - `ulimit-a.txt` — process limits on the node. Same SSH skip behavior as `df` when the node is remote and SSH was not enabled.
    - `rs.conf.txt`, `rs.status.txt`, `rs.printReplicationInfo.txt`, `rs.printSecondaryReplicationInfo.txt` — replica-set helpers on replica-set `mongod`. Skipped on **standalone** (the files record the skip). On mongos these are replaced by `sh.status.txt`.
    - `listCatalog-system.buckets.txt` — collectionless `$listCatalog` on `admin`, filtered to `system.buckets.*`, on data-bearing `mongod` (skipped on mongos, config servers, and arbiters). Catalog docs include the owning database.
    - `uniqueIndexes.txt` — non-`_id` unique indexes with WiredTiger `formatVersion` from `$collStats`. `13` or `14` is the new (post-4.2) format; anything else is listed under `oldFormat`. Same node skip as `$listCatalog`. Does **not** run `validate()`.
    - `serverStatus.shardedIndexConsistency.txt` — on a **config-server primary** only (not mongos). If the collected config member is a secondary, the file records that skip. Use `-collect-nodes=all-nodes` on a sharded cluster to include the config primary.
- Typical runtime: ~2–15 minutes depending on cluster size and network conditions.
- dcrcli does not create a cluster-level archive. After completion, compress the output directory (zip/tar.gz) yourself for upload.



## dcrcli logging

- After each execution, a log file is created in the current working directory. E.g: **dcrlogfile_1755165313.log**



## Internal Notes

- [getMongoData](https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md)
  - dcrcli invokes the mongo or mongosh shell with a compatible getMongoData.js script. Ensure the shell is in PATH. **mongosh** is preferred for consistent JSON from topology commands (`hello`, `getShardMap`, role detection).
- Node selection uses shell output to classify **PRIMARY**, **SECONDARY**, **MONGOS**, etc. Keep **mongosh** up to date for best results on sharded clusters.
- DCR command outputs are collected with the same mongosh/mongo `--eval` argv pattern as topology probes (hardcoded embedded scripts; no shell interpolation). Local `df` is `df` argv. Local `ulimit -a` is `sh -c` with a compile-time constant. Remotely, `df -h`, `df -h <dbpath>`, and `ulimit -a` run in **one** SSH session (dbpath only after the Unix path allowlist). For a remote node, OpenSSH connection sharing (`ControlMaster=auto`) reuses that session for FTDC rsync, log rsync, and the host commands so password SSH prompts once per node.
- [rsync](https://man7.org/linux/man-pages/man1/rsync.1.html)
  - Remote FTDC copy is similar to `rsync -az <ssh-username>@<hostname>:<src-path> <dest-path>`.
  - Remote mongod log copy adds `--include=<logfile>* --exclude=*`.
  - Note: The utility sequentially connects to each node, which may take time for deployments with a large number of nodes.



## Build from Source:

To build dcrcli from source, use the following commands based on your operating system: 

**Linux amd64 build steps example:**

1. Assume you are on a Linux amd64 machine
2. Clone the rep

```bash
git clone <repo-link>
```

1. Run the build:

```bash
GOOS=linux GOARCH=amd64 go build
```



## License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)

## Contributing

Pull requests are welcome. For significant changes, open an issue first to discuss scope and approach. Add or update tests where applicable.

## Security

dcrcli is a read-only collector — see [Collection Details (read-only)](#collection-details-read-only). Do not include sensitive data (credentials, PII) in issues or PRs. For security disclosures, contact maintainers privately.

## Feedback / Issues

- [https://github.com/mongodb-labs/dcrcli/issues](https://github.com/mongodb-labs/dcrcli/issues)



## DISCLAIMER

**Please note:** all tools/ scripts in this repo are released for use "AS IS" **without any warranties of any kind**,
including, but not limited to their installation, use, or performance.  We disclaim any and all warranties, either
express or implied, including but not limited to any warranty of noninfringement, merchantability, and/ or fitness
for a particular purpose.  We do not warrant that the technology will meet your requirements, that the operation
thereof will be uninterrupted or error-free, or that any errors will be corrected.

Any use of these scripts and tools is **at your own risk**.  There is no guarantee that they have been through
thorough testing in a comparable environment and we are not responsible for any damage or data loss incurred with
their use.

You are responsible for reviewing and testing any scripts you run thoroughly before use in any non-testing environment.

Thanks,
The MongoDB Support Team
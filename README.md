# dcrcli

## Description

`dcrcli` is a utility designed to collect various types of diagnostic information for a MongoDB deployment not managed by automation.

## Prerequisites

Before running `dcrcli`, ensure the following conditions are met:

1. **Network Access**: The machine running the utility must have access to the nodes and ports of the database instances.
2. **MongoDB Shell**: Either `mongo` or `mongosh` shell must be installed on the machine.
3. **Remote Log & FTDC Copy**:
   - The machine must have passwordless SSH access to all nodes of the cluster.
   - The passwordless SSH user must have read permissions for the log and FTDC files.
   - The `rsync` utility must be installed. Generally it is present as a standard utility on Linux systems.

If authentication is enabled in the deployment, the database user must have appropriate permissions (refer to the [Minimum Required Permissions](#minimum-required-permissions) section).

## Usage

Follow these steps to use `dcrcli`:

1. **Download**: Obtain the latest release from the releases link.
2. **Transfer Binary**: Copy the binary to the seed node.
   - For a replica set, the seed node can be any node.
   - For a sharded cluster, use a `mongos` instance.
3. **Set Execute Permissions**: Run the following command to give execute permissions to the binary:
   ```bash
   chmod +x <binary-name>
   ```
4. Run the binary:
   ```
   ./<binary-name>
   ```
5. Follow the on-screen prompts to complete the diagnostic data collection.

**Build from Source**
To build dcrcli from source, use the following commands based on your operating system: 
**macOS**
```
git clone <repo-link>
GOOS=linux GOARCH=amd64 go build
```

**Linux** 
```
git clone <repo-link>
go build
```

### Minimum Required Permissions

For a MongoDB deployment with authentication enabled, the database user must have the following roles:
 - backup
 - readAnyDatabase
 - clusterMonitor

These roles provide read-only access, except the backup role allows writing to two MongoDB system collections: admin.mms.backup and config.settings. The backup role is necessary for the utility to output the number of database users and user-defined roles configured.
A root/admin database user may also be used.

**Example Command**

To create a database user with the minimum required permissions, run:
```
db.getSiblingDB("admin").createUser({
    user: "ADMIN_USER",
    pwd: "ADMIN_PASSWORD",
    roles: ["backup", "readAnyDatabase", "clusterMonitor"]
});

```

### Notable Commands

Here are some of the key commands that `dcrcli` executes:

**getMongoData**

dcrcli invokes the mongo or mongosh shell with the getMongoData.js script:
getMongoData.js

**rsync**

For remote file copy tasks, dcrcli runs the rsync command with the following flags:
```
rsync -az --include=<file-pattern> --exclude=<file-pattern> --info=progress2 <ssh-username>@<hostname>:<src-path>/ <dest-path>
```

Note: The utility sequentially connects to each node, which may take time for a deployment with large number of nodes.


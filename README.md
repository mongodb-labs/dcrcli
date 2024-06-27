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

If authentication is enabled in the deployment, the database user must have appropriate permissions (refer to the [Minimum Required Permissions](https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md#more-details) section getMongoData README).

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

### License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)


DISCLAIMER
----------
Please note: all tools/ scripts in this repo are released for use "AS IS" **without any warranties of any kind**,
including, but not limited to their installation, use, or performance.  We disclaim any and all warranties, either
express or implied, including but not limited to any warranty of noninfringement, merchantability, and/ or fitness
for a particular purpose.  We do not warrant that the technology will meet your requirements, that the operation
thereof will be uninterrupted or error-free, or that any errors will be corrected.

Any use of these scripts and tools is **at your own risk**.  There is no guarantee that they have been through
thorough testing in a comparable environment and we are not responsible for any damage or data loss incurred with
their use.

You are responsible for reviewing and testing any scripts you run *thoroughly* before use in any non-testing
environment.

Thanks,  
The MongoDB Support Team

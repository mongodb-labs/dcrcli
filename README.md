# dcrcli

## Description

`dcrcli` is a command line utility to collect following types of diagnostic information for a MongoDB deployment:
 - getMongoData output for each node of the cluster
 - FTDC data for each node of the cluster
 - Logs from all nodes in the cluster


## Prerequisites

For a successful collection before running `dcrcli`, ensure the following conditions are met:

1. **Network Access** The machine running the `dcrcli` should have proper access to collect the data from all the nodes in the cluster: 
   - Hostnames of all the nodes in the mongo cluster must be resolvable from the machine running `dcrcli`.
   - Hostnames used are from mongo clusters configuration (e.g. rs.status() output)
   - Allow firewall access from machine running `dcrcli` to MongoDB instance ports e.g. 27017, 27018 etc
   - _When FTDC and mongo logs also needed:_ 
     - Allow firewall ssh access from machine running `dcrcli` to all mongo nodes in the cluster.  
2. **MongoDB Shell**: Either `mongo` or `mongosh` shell must be installed on the machine running `dcrcli` command.
3. If authentication is enabled in the deployment, the database user must have appropriate permissions (refer to the [Minimum Required Permissions](https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md#more-details) section getMongoData README).
   - If the password contains special characters like $ : / ? # [ ] @ then enter them as is percent endoing _not needed._
4. **Remote Log & FTDC Copy**:
   - The machine must have passwordless SSH access to all nodes of the cluster.
   - The passwordless SSH user must have read permissions on the mongo log and FTDC files.
   - The `rsync` utility must be installed on the machine running `dcrcli`.
   - if ssh daemon on mongo nodes uses non-default `ssh` port then use ssh config on the machine running `dcrcli` to specify the ssh port.
   - if the hostnames used on mongo nodes are not resolvable from machine running `dcrcli`, add the IP addresses of the mongo nodes to `/etc/hosts` file on the machine running `dcrcli`. 
 5. If data collection is from mongod cluster on v6 and above, the env PATH of the machine running `dcrcli` should point to mongosh binary. 
 

## Usage

Follow these steps to use `dcrcli`:

1. **Download**: Obtain the latest release from the [releases page](https://github.com/10gen/dcrcli/releases/tag/latest). 

2. **Transfer Binary**: Copy the binary to the machine that can access the MongoDB nodes.

3. **Set Execute Permissions**: Run the following command to give execute permissions to the binary:
   ```bash
   chmod +x <binary-name>
   ```
4. Run the binary:
   ```
   ./<binary-name>
   ```
5. Follow the on-screen prompts to start the data collection.

**Note:** The binary will connect to seed node which can be: 
   - For a replica set, the seed node can be any node.
   - For a sharded cluster, use a `mongos` instance.

### Internal notes

Here are some of the key commands that `dcrcli` executes:

**getMongoData**

`dcrcli` invokes the mongo or mongosh shell with the compatible version of `getMongoData.js` script. Ensure the mongo or mongosh shell is in the PATH. 

**rsync**

For remote file copy tasks, dcrcli runs the rsync command with the following flags:
```
rsync -az --include=<file-pattern> --exclude=<file-pattern> --info=progress2 <ssh-username>@<hostname>:<src-path>/ <dest-path>
```

Note: The utility sequentially connects to each node, which may take time for a deployment with large number of nodes.

**Build from Source:**
To build dcrcli from source, use the following commands based on your operating system: 

**Linux amd64 build steps example:**
1. Assume you are on a Linux amd64 machine
2. Clone the rep
```bash
git clone <repo-link>
```
3. Run the build: 
```bash
GOOS=linux GOARCH=amd64 go build
```

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

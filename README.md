# dcrcli

[![Release](https://img.shields.io/github/v/release/mongodb-labs/dcrcli?label=release)](https://github.com/mongodb-labs/dcrcli/releases)


## Description
dcrcli is a command-line utility to collect diagnostic information for MongoDB deployments:
- **getMongoData** output for each node of the cluster.
- **FTDC data** for each node of the cluster.
- **Mongod Logs** from all nodes in the cluster.

This enables centralized diagnostics and faster troubleshooting across replica sets and sharded clusters.

## Table of Contents
- [Releases](#releases)
- [Prerequisites](#prerequisites)
- [Usage](#usage)
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
- https://github.com/mongodb-labs/dcrcli/releases

## Prerequisites
For a successful collection, ensure the following before running dcrcli:

1) Network Access
- Hostnames of all nodes in the MongoDB cluster must be resolvable from the machine running dcrcli.
- Use the same hostnames as the cluster configuration (e.g., those shown by rs.status()).
- Allow firewall access from the dcrcli host to MongoDB ports (e.g., 27017, 27018, etc.).
- When FTDC and MongoDB logs are also needed:
  - Allow SSH access from the dcrcli host to all nodes in the cluster.

2) MongoDB Shell
- Either the mongo or mongosh shell must be installed on the machine running dcrcli.
- If authentication is enabled:
  - Use a database user with the appropriate permissions (see “Minimum Required Permissions” in the getMongoData README: https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md#more-details).
  - If the password contains special characters (e.g., $, /, ?, #), input them directly without percent encoding.


3) Remote Log & FTDC Copy
- The machine running dcrcli must have SSH access to all nodes in the cluster. Using [passwordless SSH](https://linodelinux.com/how-to-setup-ssh-login-without-password-in-linux/)  is recommended for an unattended run.
  - Note: rsync over SSH is used to copy files from the hosts to the dcrcli host. If passwordless SSH is not configured, a password prompt will appear for each node during collection.
- The SSH user must have read permissions on MongoDB log and FTDC files.
- Install rsync on the machine running dcrcli.
- If SSH daemons on nodes use non-default ports, specify them via SSH config on the dcrcli host.
- If hostnames used on MongoDB nodes are not resolvable, add their IP addresses to /etc/hosts on the dcrcli host.
- Optional: Ensure at least (300 × number_of_processes + 1024) MB of free space on the host running dcrcli.


4) MongoDB v6+
- Ensure the PATH includes the mongosh binary.
- Quick checks:
```
which mongosh
```
or
```
echo "$PATH"
```
## Usage
Follow these steps:
1. Download the latest release: https://github.com/mongodb-labs/dcrcli/releases
2. Transfer the binary to a machine that can access the MongoDB nodes.
3. Make it executable:
```
chmod +x <binary-name>
```

5. Run:
```
./<binary-name>
```

6. Follow the on-screen prompts to start data collection.

Terminologies:

- **Cluster Name:** Give the name of cluster to recognise easily ( APAC_PROD_RS)
- **Hostname of Seed Mongod/Mongos:** Recommended to give mongos hostname for sharded cluster, Primary hostname for replica set.
- **Port number of Seed Mongod/Mongos instance:** Port number at which mongos/mongod running on the host which we have given in previous ask.
- **Admin Username:** Admin username of database instance.
- **Admin Password:** Admin user password of database instance.
- **MongoURI options:** Any special connection string option to be specified. 
- **SSH User:** Mention the username that have SSH access to the clusters machines. Ensure that this user has read-write permissions on the dbpath of each machine.

## Output Location
- Collected artifacts are written under ./outputs.
- Typical runtime: ~2–15 minutes depending on cluster size and network conditions.
- After completion, compress the output directory (zip/tar.gz) for upload or archival.

## Internal Notes
- [getMongoData](https://github.com/mongodb/support-tools/blob/master/getMongoData/README.md)
  - dcrcli invokes the mongo or mongosh shell with a compatible getMongoData.js script. Ensure the shell is in PATH.
- [rsync](https://man7.org/linux/man-pages/man1/rsync.1.html)
  - For remote file copy tasks, dcrcli runs rsync with flags similar to:
    ```
    rsync -az --include=<file-pattern> --exclude=<file-pattern> --info=progress <ssh-username>@<hostname>:<src-path>/ <dest-path>
    ```
  - Note: The utility sequentially connects to each node, which may take time for deployments with a large number of nodes.

## Build from Source:
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

## License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)

## Contributing
Pull requests are welcome. For significant changes, open an issue first to discuss scope and approach. Add or update tests where applicable.

## Security
Do not include sensitive data (credentials, PII) in issues or PRs. For security disclosures, contact maintainers privately.

## Feedback / Issues
- https://github.com/mongodb-labs/dcrcli/issues

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

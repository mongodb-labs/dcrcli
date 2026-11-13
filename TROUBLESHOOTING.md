### Description
This document helps with common troubleshooting scenarios.

### Troubleshooting Remote Copy of Mongod Logs and FTDC Failures due to Known Hosts Error

* **Understanding the `known_hosts` file**: The `known_hosts` file is a security feature used by SSH (Secure Shell) to verify the identity of remote hosts. It stores the public keys of all known hosts, which are used to authenticate connections. dcrcli never turns this check off, and neither should you.
* **Step 1: Check whether the host is already known**
        + Check that `~/.ssh/known_hosts` exists and has an entry for the node: `ssh-keygen -F <hostname>`.
        + If the node is missing, go to Step 2. If it is present and SSH reports `REMOTE HOST IDENTIFICATION HAS CHANGED`, go to Step 3.
* **Step 2: Add a host you have not connected to before (verify first)**
        + Connect once from a terminal: `ssh <user>@<hostname>`.
        + SSH prints the host's key fingerprint and asks whether to continue. Compare it with a fingerprint you obtained from a trusted source, for example by running `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the node itself or asking whoever manages the node. Answer `yes` only if they match.
        + In the web UI the same question appears as a prompt in the browser. Check the fingerprint the same way before you accept it.
        + To record many hosts at once, collect their keys from a source you trust and add them to `known_hosts`. Do not use an unverified `ssh-keyscan` result as proof of identity.
* **Step 3: A host key that has changed**
        + A changed key means the node was rebuilt or re-imaged, **or** that something is intercepting the connection. Do not just bypass it.
        + Confirm with the node's owner that the key changed on purpose, and verify the new fingerprint as in Step 2.
        + Only then remove the stale entry and reconnect: `ssh-keygen -R <hostname>`, then `ssh <user>@<hostname>`.
* **Step 4: Test the connection**: Run `ssh <user>@<hostname> true` and confirm it completes without any host-key warning.

#### What not to do

Do not add `StrictHostKeyChecking no` or `UserKnownHostsFile /dev/null` to `~/.ssh/config` (and especially not under `Host *`). They disable host-key verification for every SSH connection from your account, so a man-in-the-middle can impersonate a node and receive the credentials and diagnostic data dcrcli sends to it. If a single lab host really must be exempt, scope the exception to that one hostname rather than `Host *`, and remove it when you are done.

### Web UI

* **`dcr web` says a port is already in use, or `-via` prints a URL that does not respond**: another program owns the local port. `-via` exits when SSH cannot bind its local port. Choose another one with `-local-port`.
* **Host key changed in the browser prompt**: follow Step 3 above. The web UI explains the mismatch but never overrides it.
* **A cluster member's hostname cannot be resolved**: the web UI needs every member to resolve on the machine where dcr runs. Add the names to DNS or `/etc/hosts` as the error message shows.

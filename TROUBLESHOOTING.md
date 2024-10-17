### Description
This documents helps with common troubleshooting scenarios

### Troubleshooting Remote Copy of Mongod Logs and FTDC Failures due to Known Hosts Error

* **Understanding the `known_hosts` file**: The `known_hosts` file is a security feature used by SSH (Secure Shell) to verify the identity of remote hosts. It stores the public keys of all known hosts, which are used to authenticate connections.
* **Step 1: Verify the `known_hosts` file**
        + Ensure that the `known_hosts` file is properly populated with the nodes you're trying to connect to.
        + Check if the file exists in the default location (`~/.ssh/known_hosts`) and contains the public keys of all expected hosts.
* **Step 2: Configure SSH to skip known hosts checking (Workaround)**   
        + Edit the `~/.ssh/config` file using a text editor (e.g., `nano` or `vim`).
        + Add the following lines to the end of the file:  
```bash
     Host *
         UserKnownHostsFile /dev/null
         StrictHostKeyChecking no
```   

* **Step 3: Test the connection:** Try running ssh on remote nodes to see that it does not prompt for known_hosts. 

Note: This workaround disables strict host key checking, which may compromise security. It's recommended to properly populate the `known_hosts` file or use a more secure alternative solution.


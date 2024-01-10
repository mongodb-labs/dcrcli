1. Launch a ec2 instance from aws console (launched from corp.mongodb.com)
2. Add tags:
```
    name: instance-name
    owner: nishant.bhardwaj
    keep_until: yyyy-mm-dd 
```
3. choose ubuntu 20
4. choose key nbssh
5. add the ip address for ssh
6. note the public ip4 address
7. ssh -i ~/.ssh/nbssh.pem ubuntu@<public ipv4 address>
sudo apt-get update   
sudo apt-get upgrade
sudo chown ubuntu:ubuntu /usr/local/lib
sudo chown ubuntu:ubuntu /usr/local/bin
sudo apt-get install m
sudo apt install python3-pip
pip3 install --user 'mtools[all]'
m 4.2.24

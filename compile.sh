#cross compile for Linux: GOOS=linux GOARCH=amd64 go build
go build; mv dcrcli dcrcli-osx; GOOS=linux GOARCH=amd64 go build ; mv dcrcli dcrcli-linux
scp -i ~/.ssh/nbssh.pem dcrcli* ubuntu@3.110.159.159:
scp -i ~/.ssh/nbssh.pem rundcr.sh ubuntu@3.110.159.159:
ssh -i ~/.ssh/nbssh.pem ubuntu@3.110.159.159

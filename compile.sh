#cross compile for Linux: GOOS=linux GOARCH=amd64 go build
go build; mv dcrcli dcrcli-osx; GOOS=linux GOARCH=amd64 go build ; mv dcrcli dcrcli-linux
ssh -i ~/.ssh/nbssh.pem ubuntu@15.207.113.51 mkdir -p /home/ubuntu/dcrcli/outputs /home/ubuntu/dcrcli/assets/getMongoData /home/ubuntu/dcrcli/assets/mongoWellnessChecker
scp -i ~/.ssh/nbssh.pem dcrcli* ubuntu@15.207.113.51:/home/ubuntu/dcrcli
scp -i ~/.ssh/nbssh.pem rundcr.sh ubuntu@15.207.113.51:/home/ubuntu/dcrcli
scp -i ~/.ssh/nbssh.pem ./assets/getMongoData/getMongoData.js ubuntu@15.207.113.51:/home/ubuntu/dcrcli/assets/getMongoData/
scp -i ~/.ssh/nbssh.pem ./assets/mongoWellnessChecker/mongoWellnessChecker.js ubuntu@15.207.113.51:/home/ubuntu/dcrcli/assets/mongoWellnessChecker
ssh -i ~/.ssh/nbssh.pem ubuntu@15.207.113.51

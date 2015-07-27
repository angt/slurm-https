# slurm-https

**This project is an abandonned PoC.**

**Never used, never tested !**

I put the code here, maybe one day it will be helpful to someone.

## Quick Start

### Build
```sh
$ git clone https://github.com/angt/slurm-https.git
$ cd slurm-https
$ go build
```

### Run the server
```sh
$ ./create-cert.sh
$ ./slurm-https
```

## Test it with cURL

### Get the conf
```sh
$ curl --cert ./client.crt --cacert ./ca.crt --key ./client.key --insecure -d @- https://localhost:8443/conf <<EOF
{
    "UpdateTime":0
}
EOF
```

### Submit a batch job
```sh
$ curl --cert ./client.crt --cacert ./ca.crt --key ./client.key --insecure -d @- https://localhost:8443/job/submit <<EOF
{                 
    "Name":"test",
    "TimeLimit":200,
    "MinNodes":1,
    "UserId":$(id -u),
    "GroupId":$(id -g),
    "WorkDir":"${HOME}",
    "Script":"#!/bin/sh\nhostname\n"
}                                   
EOF
```

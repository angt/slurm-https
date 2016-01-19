# slurm-https

A simple HTTPS API for Slurm.

## Quick Start

### Build
```sh
$ git clone https://github.com/angt/slurm-https.git
$ cd slurm-https
$ go build
```

### Run
```sh
$ ./create-cert.sh
$ ./slurm-https
```
By default, the server listen on `:8443`.

## API

The API is nearly a direct mapping to [slurm.h](https://raw.githubusercontent.com/SchedMD/slurm/master/slurm/slurm.h.in).

 endpoint           | description
--------------------|------------------------------------------------------------------------------------
/nodes              | get all node configuration information if changed since UpdateTime
/node/update        | update node's configuration (root only)
/licenses           | get license information
/conf               | get control configuration information if changed since UpdateTime
/jobs               | get all job configuration information if changed since UpdateTime
/job/alloc          | allocate resources for a job request
/job/submit         | submit a job for later execution
/job/lookup         | get info for an existing resource allocation
/job/update         | update job's configuration
/job/notify         | send message to the job's stdout (root only)
/job/kill           | send the specified signal to all steps of an existing job (with flags)
/job/signal         | send the specified signal to all steps of an existing job
/job/complete       | note the completion of a job and all of its steps
/job/suspend        | suspend execution of a job
/job/resume         | resume execution of a previously suspended job
/job/requeue        | re-queue a batch job, if already running then terminate it first
/job/step/kill      | send the specified signal to an existing job step
/job/step/signal    | send the specified signal to an existing job step
/job/step/terminate | terminates a job step
/frontends          | get all frontend configuration information if changed since UpdateTime
/frontend/update    | update frontend node's configuration (root only)
/topologies         | get all switch topology configuration information
/partitions         | get all partition configuration information if changed since UpdateTime
/partition/create   | create a new partition (root only)
/partition/update   | update a partition's configuration (root only)
/partition/delete   | delete a partition (root only)
/reservations       | get all reservation configuration information if changed since UpdateTime
/reservation/create | create a new reservation (root only)
/reservation/update | update a reservation's configuration (root only)
/reservation/delete | delete a reservation (root only)
/triggers           | get all event trigger information
/trigger/create     | create an event trigger
/trigger/delete     | delete an event trigger
/ping               | ping the slurm controller
/reconfigure        | force the slurm controller to reload its configuration file
/shutdown           | shutdown the slurm controller
/takeover           | force the slurm backup controller to take over the primary controller

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

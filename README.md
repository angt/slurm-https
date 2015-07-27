# slurm-https

This project is an abandonned PoC.
Never used, never tested!

I put the code here, maybe one day it will be helpful to someone.

## Build
    ```sh
    $ git clone https://github.com/angt/slurm-https.git
    $ cd slurm-https
    $ go build
    ```

## Run the server
    ```sh
    $ ./create-cert.sh
    $ ./slurm-https
    ```

## Get the conf
    ```sh
    $ curl --cert ./client.crt --cacert ./ca.crt --key ./client.key --insecure -d '{"UpdateTime":0}' https://localhost:8443/conf
    ```

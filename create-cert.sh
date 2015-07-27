#!/bin/bash

cat >openssl.conf<<EOF
[ ssl_client ]
    extendedKeyUsage = clientAuth
EOF

cat >ca.srl<<EOF
00
EOF

subject="/C=/ST=/L=/O=/OU=/emailAddress="

#ca
openssl req -new -newkey rsa:2048 -x509 -days 365 -out ca.crt -keyout ca.key -subj ${subject}"/CN=ca"

#server
openssl req -new -newkey rsa:2048 -nodes -out server.csr -keyout server.key -subj ${subject}"/CN=$(hostname)"
openssl x509 -req -days 365 -in server.csr -out server.crt -CA ca.crt -CAkey ca.key -CAserial ca.srl

#client
openssl req -new -newkey rsa:2048 -nodes -out client.csr -keyout client.key -subj ${subject}"/CN=client"
openssl x509 -extfile openssl.conf -extensions ssl_client -req -days 365 -in client.csr -out client.crt -CA ca.crt -CAkey ca.key -CAserial ca.srl

chmod 600 *.key

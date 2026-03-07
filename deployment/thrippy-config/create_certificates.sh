#!/bin/bash

# CA
openssl req -x509                                    \
  -newkey rsa:4096                                   \
  -noenc                                             \
  -days 3650                                         \
  -out thrippy_ca_cert.pem                           \
  -keyout thrippy_ca_key.pem                         \
  -subj /C=US/ST=CA/O=Tzrikka/OU=Thrippy/CN=test-ca/ \
  -config ./openssl.cnf                              \
  -extensions test_ca                                \
  -sha256

# Server
openssl genrsa -out thrippy_server_key.pem 4096

openssl req -new                                         \
  -key thrippy_server_key.pem                            \
  -out thrippy_server_csr.pem                            \
  -subj /C=US/ST=CA/O=Tzrikka/OU=Thrippy/CN=test-server/ \
  -config ./openssl.cnf                                  \
  -reqexts test_server

openssl x509 -req              \
  -in thrippy_server_csr.pem   \
  -CA thrippy_ca_cert.pem      \
  -CAkey thrippy_ca_key.pem    \
  -days 3650                   \
  -out thrippy_server_cert.pem \
  -extfile ./openssl.cnf       \
  -extensions test_server      \
  -CAcreateserial              \
  -sha256

openssl verify -verbose -CAfile thrippy_ca_cert.pem thrippy_server_cert.pem

# Client
openssl genrsa -out thrippy_client_key.pem 4096

openssl req -new                                         \
  -key thrippy_client_key.pem                            \
  -out thrippy_client_csr.pem                            \
  -subj /C=US/ST=CA/O=Tzrikka/OU=Thrippy/CN=test-client/ \
  -config ./openssl.cnf                                  \
  -reqexts test_client

openssl x509 -req              \
  -in thrippy_client_csr.pem   \
  -CA thrippy_ca_cert.pem      \
  -CAkey thrippy_ca_key.pem    \
  -days 3650                   \
  -out thrippy_client_cert.pem \
  -extfile ./openssl.cnf       \
  -extensions test_client      \
  -CAcreateserial              \
  -sha256

openssl verify -verbose -CAfile thrippy_ca_cert.pem thrippy_client_cert.pem

rm *_csr.pem
chmod 600 *.pem
cp -f thrippy_c* ../revchat-config/
cp -f thrippy_c* ../timpani-config/

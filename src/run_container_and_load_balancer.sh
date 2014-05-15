#!/usr/bin/env bash

# build docker image from Dockerfile
docker build -t samirarabbanian/netty .

# make sure any containers that may exist from a previous run are stopped and removed
docker stop netty_1080 2> /dev/null
docker stop netty_1081 2> /dev/null
docker rm netty_1080 2> /dev/null
docker rm netty_1081 2> /dev/null

# run two instances of the container mapped to ports 1080 and 1081
docker run -d -name netty_1080 -p 1080:8080 samirarabbanian/netty
docker run -d -name netty_1081 -p 1081:8080 samirarabbanian/netty

# create ipc socket file
mkdir -p /tmp/feeds

# run go script that uses ZeroMQ to load balance between the two instances (logging level at info)
/usr/local/go/bin/go build -o http_zmq_load_balancer http_zmq_load_balancer.go
./http_zmq_load_balancer 2 9090 1080 1081 &

# confirm everything has started correctly
curl http://localhost:9090/zeromq
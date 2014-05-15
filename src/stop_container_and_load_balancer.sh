#!/usr/bin/env bash

# stop two containers instances
docker stop netty_1080
docker stop netty_1081

# delete containers
docker rm netty_1080
docker rm netty_1081

# stop ZeroMQ load balancer
killall http_zmq_load_balancer
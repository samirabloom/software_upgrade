ZeroMQ Load Balancer
====================

This project demonstrates using ZeroMQ to load balance between two docker containers running a very simple Netty web server

The following diagram shows the interaction:

![Overview Diagram](https://github.com/samirarabbanian/software_upgrade/raw/master/overviewDiagram_2014_02_14.png)

To start docker and the ZeroMQ load balancer use:

    ./run_container_and_load_balancer.sh

To stop docker and the ZeroMQ load balancer use:

    ./stop_container_and_load_balancer.sh

**Note:** The TestServer.java is added here although this is downloaded to the docker container from github at https://github.com/samirarabbanian/netty_example/blob/master/TestServer.java

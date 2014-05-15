package main

import (
	"os"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"time"
	"regexp"
	"strconv"
)

const (
	REQUEST_TIMEOUT = 10 * time.Millisecond //  msecs
)

// define ZMQ response handler
func main() {
	fmt.Println("\nForwarding: " + os.Args[1] + " -> " + os.Args[2] + "\n")

	go upstream(os.Args[1])

	//  Run for 2 minutes then quit
	time.Sleep(120 * time.Second)
}

func upstream(upstreamPort string) {
	//  socket for incoming HTTP request/response
	upstream, error := zmq.NewSocket(zmq.STREAM)
	if error != nil {
		fmt.Printf("error creating upstream socket: %s\n", error)
	} else {
		defer upstream.Close()
		fmt.Println("created upstream socket")
	}
	error = upstream.Bind("tcp://*:"+upstreamPort)
	if error != nil {
		fmt.Printf("error binding upstream socket: %s\n", error)
	} else {
		fmt.Println("bound upstream socket to [" + upstreamPort + "]")
	}

	//  inproc socket for sending requests to downstream
	backend, error := zmq.NewSocket(zmq.REQ)
	if error != nil {
		fmt.Printf("error creating backend socket: %s\n", error)
	} else {
		defer backend.Close()
		fmt.Println("created backend socket")
	}
	error = backend.Connect("ipc:///tmp/feeds/0")
	if error != nil {
		fmt.Printf("error binding backend socket: %s\n", error)
	} else {
		fmt.Println("bound backend socket to [ipc:///tmp/feeds/0]")
	}

	//	//  Connect backend to frontend via a proxy
	//	err := zmq.Proxy(upstream, backend, nil)
	//	fmt.Println("Fronend Proxy interrupted:", err)

	upstreamPoller := zmq.NewPoller()
	upstreamPoller.Add(upstream, zmq.POLLIN)

	backendPoller := zmq.NewPoller()
	backendPoller.Add(backend, zmq.POLLIN)

	id := ""
	downstreamStarted := false
	for {
		polled, error := upstreamPoller.Poll(REQUEST_TIMEOUT)
		if (!downstreamStarted) {
			downstreamStarted = true
			go downstream(os.Args[2])
		}

		if error == nil && len(polled) == 1 {
			id, error = upstream.Recv(0)
			if error != nil {
				fmt.Printf("error receiveing request id from upstream: %s\n", error)
			} else {
				fmt.Printf("\n\nreceived upstream request id message [% x]\n", id)

				// send request to backend
				backend.Send(id, zmq.SNDMORE)

				fmt.Println("send backend")
			}
			message, error := upstream.Recv(0)
			if error != nil {
				fmt.Printf("error receiveing request from upstream: %s\n", error)
			} else {
				fmt.Println("received request from upstream [\n" + message + "]")

				// send request to backend
				backend.Send(message, 0)

				fmt.Println("send backend")
			}
		}

		polled, error = backendPoller.Poll(REQUEST_TIMEOUT)
		if error == nil && len(polled) == 1 {
			message, error := backend.Recv(0)
			if error != nil {
				fmt.Printf("error receiveing response from backend: %s\n", error)
			} else {
				fmt.Println("received response from backend [\n" + message + "]")
				if (message != "CLOSE") {
					// send response upstream
					upstream.Send(id, zmq.SNDMORE)
					upstream.Send(message, zmq.SNDMORE)
				} else {
					// send response upstream
					upstream.Send(id, zmq.SNDMORE)
					upstream.Send("", 0)

					downstreamStarted = false;

					fmt.Println("CLOSING")
				}
			}
		}
	}
}

func downstream(downstreamHostAndPort string) {
	// socket for downstream HTTP request/response
	downstream, error := zmq.NewSocket(zmq.STREAM)
	if error != nil {
		fmt.Printf("error creating downstream socket: %s\n", error)
	} else {
		defer downstream.Close()
		fmt.Println("created downstream socket")
	}
	error = downstream.Connect("tcp://"+downstreamHostAndPort)
	if error != nil {
		fmt.Printf("error binding downstream socket: %s\n", error)
	} else {
		fmt.Println("bound downstream socket to [" + downstreamHostAndPort + "]")
	}

	// inproc socket for receiving requests from frontend
	frontend, error := zmq.NewSocket(zmq.REP)
	if error != nil {
		fmt.Printf("error creating frontend socket: %s\n", error)
	} else {
		defer frontend.Close()
		fmt.Println("created frontend socket")
	}
	error = frontend.Bind("ipc:///tmp/feeds/0")
	if error != nil {
		fmt.Printf("error binding frontend socket: %s\n", error)
	} else {
		fmt.Println("bound frontend socket to [ipc:///tmp/feeds/0]")
	}

	// receive id
	id, error := frontend.Recv(0)
	if error != nil {
		fmt.Printf("error receiveing id from frontend: %s\n", error)
	} else {
		fmt.Printf("\n\nreceived frontend id message [% x]\n", id)
	}

	// receive message
	message, error := frontend.Recv(0)
	if error != nil {
		fmt.Printf("error receiveing message from frontend: %s\n", error)
	} else {
		fmt.Println("received message from frontend [\n" + message + "]")

		// disable compression to simplify proxy
		message = regexp.MustCompile("Accept-Encoding: [a-z\\,]*").ReplaceAllLiteralString(message, "")
		// disable persistent connections to simplify proxy
		// this avoids HTTP response chunks from arriving in the same ZeroMQ message for different requests
		message = regexp.MustCompile("Connection: keep-alive").ReplaceAllLiteralString(message, "Connection: close")

		// ensure that the request has the correct Host header
		message = regexp.MustCompile("Host: [a-z\\,\\.\\:\\d]*").ReplaceAllLiteralString(message, "Host: "+downstreamHostAndPort)


		id, error := downstream.GetIdentity()
		if error != nil {
			fmt.Printf("error getting server identity: %s\n", error)
		} else {
			fmt.Printf("received server identity [% x]\n", id)
		}

		fmt.Printf("\nsending id downstream [% x]\n", id)
		fmt.Println("sending message downstream [\n" + message + "]")

		// send id message
		downstream.Send(id, zmq.SNDMORE)
		// send response message - 1st chunk
		downstream.Send(message, zmq.SNDMORE)

		contentReceived := 0
		contentLength := 0
		contentLengthHeader := ""
		transferEncodingHeader := ""
		for {
			// receive id
			id, error := downstream.Recv(0)
			if error != nil {
				fmt.Printf("error receiveing downstream response id message: %s\n", error)
			} else {
				fmt.Printf("\n\nreceived downstream response id message [% x]\n", id)
			}

			// receive message
			message, error := downstream.Recv(0)
			if error != nil {
				fmt.Printf("error receiveing downstream response message: %s\n", error)
			} else {
				fmt.Println("received downstream message [\n" + message + "]")

				// send message to frontend
				frontend.Send(message, zmq.SNDMORE)
			}

			if len(contentLengthHeader) == 0 && len(transferEncodingHeader) == 0 {
				contentLengthHeader = regexp.MustCompile("Content-Length: \\d+").FindString(message)
				contentLength, _ = strconv.Atoi(regexp.MustCompile("\\d+").FindString(contentLengthHeader))
				fmt.Println("ContentLengthHeader: " + contentLengthHeader)
				transferEncodingHeader = regexp.MustCompile("Transfer-Encoding: chunked").FindString(message)
				fmt.Println("TransferEncodingHeader: " + transferEncodingHeader)
			}

			if len(contentLengthHeader) > 0 {

				if (contentReceived == 0) {
					// first chunk with headers
					for i := 0; i < len(message); i++ {
						if (i >= 4 &&
							string(message[i - 4]) == string("\u000D") &&
							string(message[i - 3]) == string("\u000A") &&
							string(message[i - 2]) == string("\u000D") &&
							string(message[i - 1]) == string("\u000A")) {
							// start of body (end of headers)
							contentReceived += (len(message)-i)
						}
					}

				} else {
					// not first chunk so no header
					contentReceived += len(message)
				}

				fmt.Printf("contentReceived: %d\n", contentReceived)
				fmt.Printf("contentLength: %d\n", contentLength)

				if contentReceived >= contentLength {
					// received all content
					break;
				}

			} else if len(transferEncodingHeader) > 0 {
				// Transfer-Encoding: chunked

				// check if downstream has sent final empty chunk (as per HTTP specification)
				if len(message) == 0 {
					// received final chunk
					break;
				}

				//				// debug final chunk detection
				//				fmt.Print("string(message[len(message) - 5]) == string(\"u0030\"): "); fmt.Println(string(message[len(message) - 5]) == string("\u0030"))
				//				fmt.Print("string(message[len(message) - 4]) == string(\"u000D\"): "); fmt.Println(string(message[len(message) - 4]) == string("\u000D"))
				//				fmt.Print("string(message[len(message) - 3]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 3]) == string("\u000A"))
				//				fmt.Print("string(message[len(message) - 2]) == string(\"u000D\"): "); fmt.Println(string(message[len(message) - 2]) == string("\u000D"))
				//				fmt.Print("string(message[len(message) - 1]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 1]) == string("\u000A"))

				// check if downstream sends trailer indicating final chunk, as follows:
				// -5 U+0030 - '0'
				// -4 U+000D - Carriage return
				// -3 U+000A - Line feed
				// -2 U+000D - Carriage return
				// -1 U+000A - Line feed
				// this allows for downstream which doesn't conform strictly to HTTP specification
				length := len(message)
				if (string(message[length - 5]) == string("\u0030") &&
					string(message[length - 4]) == string("\u000D") &&
					string(message[length - 3]) == string("\u000A") &&
					string(message[length - 2]) == string("\u000D") &&
					string(message[length - 1]) == string("\u000A")) {
					// received final chunk
					break;
				}
			} else {
				fmt.Println("Error can't find either Content-Length or Transfer-Encoding headers")
			}
		}

		// close connection

		// send id message
		downstream.Send(id, zmq.SNDMORE)
		// send empty chunk
		downstream.Send("", 0)

		// send CLOSE instruction to frontend
		frontend.Send("CLOSE", 0)

		fmt.Println("connection closed")
	}
}

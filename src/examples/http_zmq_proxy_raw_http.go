package main

import (
	"os"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"regexp"
	"strconv"
)

// define ZMQ response handler
func main() {
	fmt.Println("\nForwarding: " + os.Args[1] + " -> " + os.Args[2] + "\n")
	sendDownstream := downstream(os.Args[2])
	upstream(os.Args[1], sendDownstream)
}

func upstream(upstreamPort string, sendDownstream func(message string, sendUpstream func(response string), closeUpstream func())) {
	upstream, error := zmq.NewSocket(zmq.STREAM)
	if error != nil {
		fmt.Printf("error creating upstream socket: %s\n", error)
	} else {
		fmt.Println("created upstream socket")
	}

	error = upstream.Bind("tcp://*:"+upstreamPort)
	if error != nil {
		fmt.Printf("error binding upstream socket: %s\n", error)
	} else {
		fmt.Println("bound upstream socket to [" + upstreamPort + "]")
	}

	for {
		// receive id
		id, error := upstream.Recv(0)
		if error != nil {
			fmt.Printf("error receiveing upstream message: %s\n", error)
		} else {
			fmt.Printf("\n\nreceived upstream id message [% x]\n", id)
		}

		// receive message
		message, error := upstream.Recv(0)
		if error != nil {
			fmt.Printf("error receiveing upstream message: %s\n", error)
		} else {
			fmt.Println("received message [\n" + message + "]")

			// closure for sending response upstream
			sendUpstream := func(message string) {
				// send id message
				upstream.Send(id, zmq.SNDMORE)
				// send response message - 1st chunk
				upstream.Send(message, zmq.SNDMORE)
			}
			// closure for closing connection
			closeUpstream := func() {
				// send id message
				upstream.Send(id, zmq.SNDMORE)
				// send close response by sending empty chunk
				upstream.Send("", zmq.SNDMORE)
			}
			sendDownstream(message, sendUpstream, closeUpstream)
		}
	}
}

func downstream(downstreamHostAndPort string) func(message string, sendUpstream func(response string), closeUpstream func()) {

	return func(message string, sendUpstream func(response string), closeUpstream func()) {
		downstream, error := zmq.NewSocket(zmq.STREAM)
		if error != nil {
			fmt.Printf("error creating downstream socket: %s\n", error)
		} else {
			fmt.Println("created downstream socket")
		}

		error = downstream.Connect("tcp://"+downstreamHostAndPort)
		if error != nil {
			fmt.Printf("error binding downstream socket: %s\n", error)
		} else {
			fmt.Println("bound downstream socket to [" + downstreamHostAndPort + "]")
		}

		id, error := downstream.GetIdentity()
		if error != nil {
			fmt.Printf("error getting server identity: %s\n", error)
		} else {
			fmt.Printf("received server identity [% x]\n", id)
		}

		// disable compression to simplify proxy
		message = regexp.MustCompile("Accept-Encoding: [a-z\\,]*").ReplaceAllLiteralString(message, "")
		// disable persistent connections to simplify proxy
		// this avoids HTTP response chunks from arriving in the same ZeroMQ message for different requests
		message = regexp.MustCompile("Connection: keep-alive").ReplaceAllLiteralString(message, "Connection: close")

		// ensure that the request has the correct Host header
		message = regexp.MustCompile("Host: [a-z\\,\\.\\:\\d]*").ReplaceAllLiteralString(message, "Host: " + downstreamHostAndPort)


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
				fmt.Printf("error receiveing downstream message: %s\n", error)
			} else {
				fmt.Printf("\n\nreceived downstream id message [% x]\n", id)
			}

			// receive message
			message, error := downstream.Recv(0)
			if error != nil {
				fmt.Printf("error receiveing downstream message: %s\n", error)
			} else {
				fmt.Println("received downstream message [\n" + message + "]")
				sendUpstream(message)
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
		downstream.Send("", zmq.SNDMORE)

		closeUpstream();

		fmt.Println("connection closed")
	}
}

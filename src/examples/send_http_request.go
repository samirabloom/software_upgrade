package main

import (
	"fmt"
	zmq "github.com/pebbe/zmq4"
)

func main() {
	socket, error := zmq.NewSocket(zmq.STREAM)
	if error != nil {
		fmt.Printf("error creating downstream socket: %s\n", error)
	} else {
		fmt.Println("created downstream socket")
	}

	error = socket.Connect("tcp://www.mock-server.com:80")
	if error != nil {
		fmt.Printf("error binding downstream socket: %s\n", error)
	} else {
		fmt.Println("bound downstream socket to [www.mock-server.com:80]")
	}

	id, error := socket.GetIdentity()
	if error != nil {
		fmt.Printf("error getting server identity: %s\n", error)
	} else {
		fmt.Printf("received server identity [% x]\n", id)
	}

	// send GET request

	// send id message
	socket.Send(id, zmq.SNDMORE)
	// send empty chunk
	socket.Send("GET /doc/articles/wiki/ HTTP/1.1\r\n" +
				"Host: www.mock-server.com\r\n" +
				"Accept: */*\r\n" +
				"\r\n", zmq.SNDMORE)

	for {
		// receive id
		id, error := socket.Recv(0)
		fmt.Println("2")
		if error != nil {
			fmt.Printf("error receiveing downstream message: %s\n", error)
		} else {
			fmt.Printf("\n\nreceived downstream id message [% x]\n", id)
		}

		// receive message
		message, error := socket.Recv(0)
		if error != nil {
			fmt.Print("error receiveing downstream message: %s\n", error)
		} else {
			fmt.Println("received downstream message [\n" + message + "]")
		}

		// check if downstream has sent final empty chunk (as per HTTP specification)
		if len(message) == 0 {
			break;
		}

		//		// debug final chunk detection
		//		fmt.Print("string(message[len(message) - 8]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 9]) == string("\u000A"))
		//		fmt.Print("string(message[len(message) - 7]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 8]) == string("\u000A"))
		//		fmt.Print("string(message[len(message) - 6]) == string(\"u000D\"): "); fmt.Println(string(message[len(message) - 7]) == string("\u000D"))
		//		fmt.Print("string(message[len(message) - 5]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 6]) == string("\u000A"))
		//		fmt.Print("string(message[len(message) - 4]) == string(\"u0030\"): "); fmt.Println(string(message[len(message) - 5]) == string("\u0030"))
		//		fmt.Print("string(message[len(message) - 3]) == string(\"u000D\"): "); fmt.Println(string(message[len(message) - 4]) == string("\u000D"))
		//		fmt.Print("string(message[len(message) - 2]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 3]) == string("\u000A"))
		//		fmt.Print("string(message[len(message) - 1]) == string(\"u000D\"): "); fmt.Println(string(message[len(message) - 2]) == string("\u000D"))
		//		fmt.Print("string(message[len(message) - 0]) == string(\"u000A\"): "); fmt.Println(string(message[len(message) - 1]) == string("\u000A"))

		// check if downstream sends trailer indicating final chunk, as follows:
		// -8 U+000A - Line feed
		// -7 U+000A - Line feed
		// -6 U+000D - Carriage return
		// -5 U+000A - Line feed
		// -4 U+0030 - '0'
		// -3 U+000D - Carriage return
		// -2 U+000A - Line feed
		// -1 U+000D - Carriage return
		// -0 U+000A - Line feed
		// this allows for downstream which doesn't conform strictly to HTTP specification
		length := len(message)
		if (string(message[length - 9]) == string("\u000A") &&
			string(message[length - 8]) == string("\u000A") &&
			string(message[length - 7]) == string("\u000D") &&
			string(message[length - 6]) == string("\u000A") &&
			string(message[length - 5]) == string("\u0030") &&
			string(message[length - 4]) == string("\u000D") &&
			string(message[length - 3]) == string("\u000A") &&
			string(message[length - 2]) == string("\u000D") &&
			string(message[length - 1]) == string("\u000A")) {
			break;
		}
	}

	// close connection

	// send id message
	socket.Send(id, zmq.SNDMORE)
	// send empty chunk
	socket.Send("", zmq.SNDMORE)

	fmt.Println("connection closed")
}


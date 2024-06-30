package main

import (
	"fmt"
	"net"

	grpc "google.golang.org/grpc"
)

func main() {
	fmt.Println("Strike")

	serviceInitializer := &grpc.ServiceDesc{
		ServiceName: "Strike_foundation",
	}

	srvr := grpc.NewServer()
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("Uh Oh: %v\n", err)
	}

	srvr.RegisterService(serviceInitializer, nil)
	srvr.Serve(listener)

  listener.Accept()
  listener.Addr()

  lsrvr := len(srvr.GetServiceInfo())

  fmt.Println(lsrvr)

  
  
  fmt.Println("Im listening")
}

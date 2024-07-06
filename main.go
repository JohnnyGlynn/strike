package main

import (
	"fmt"
	"net"

	network "github.com/JohnnyGlynn/strike/src"
  
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
  err = srvr.Serve(listener)
  if err != nil{
    fmt.Println("Error%v\n", err)
  }

  network.Scan_Network()
  
  listener.Accept()
  listener.Addr()

  lsrvr := len(srvr.GetServiceInfo())

  fmt.Println(lsrvr)

  
  
  fmt.Println("Im listening")
}

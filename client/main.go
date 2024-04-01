package main

import (
	"fmt"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	fmt.Println("Strike client")

	// context for later
	// ctx, close := context.WithTimeout(context.Background(), time.Second)
	// defer close()

	trgt := "localhost:8080"

	clynt, err := grpc.Dial(trgt, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("Uh Oh: %v\n", err)
	}

	fmt.Printf("Connected to server: %v\n", clynt.Target())
	defer clynt.Close()
}

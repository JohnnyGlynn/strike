package main

import (
	"fmt"
	"os"

	"github.com/JohnnyGlynn/strike/internal/client"
	"github.com/JohnnyGlynn/strike/internal/keys"
)

func main() {
	fmt.Println("Strike client")

	//check if its the first startup then create a userfile and generate keys
	_, err := os.Stat("./cfg/userfile")
	if os.IsNotExist(err) {
		fmt.Println("First time setup")

		//create config directory
		direrr := os.Mkdir("./cfg", 0755)
		if direrr != nil {
			fmt.Println("Error creating directory:", direrr)
			return
		}

		//create userfile
		_, ferr := os.Create("./cfg/userfile")
		if ferr != nil {
			fmt.Println("Error creating userfile:", ferr)
			os.Exit(1)
		}
		//key generation
		pubpath := keys.Keygen()

		client.RegisterClient(pubpath)
		client.Login("client0", pubpath)
		client.AutoChat()

	} else {
		client.AutoChat()
	}

}

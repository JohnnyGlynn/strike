package client

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/JohnnyGlynn/strike/internal/client/crypto"
	"github.com/JohnnyGlynn/strike/internal/client/network"
	"github.com/JohnnyGlynn/strike/internal/client/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

func printPrompt(client *types.ClientInfo) {
	switch client.Shell.Mode {
	case types.ModeDefault:
		fmt.Printf("[shell:%s]> ", client.Username)
	case types.ModeChat:
		fmt.Printf("[chat:%s@%s]> ", client.Username, client.Cache.CurrentChat.User.Name)
	}
}

func inputParse(input string) types.ParsedInput {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "/") {
		parts := strings.Fields(input)
		if len(parts) == 0 {
			return types.ParsedInput{}
		}
		return types.ParsedInput{
			IsCommand: true,
			Command:   parts[0],
			Args:      parts[1:],
			Raw:       input,
		}
	}
	return types.ParsedInput{
		IsCommand: false,
		Raw:       input,
	}
}

func dispatchCommand(cmdMap map[string]types.Command, parsed types.ParsedInput, client *types.ClientInfo) {
	if cmd, exists := cmdMap[parsed.Command]; exists {
		if slices.Contains(cmd.Scope, client.Shell.Mode) {
			cmd.CmdFn(parsed.Args, client)
		} else {
			fmt.Printf("'%s' command not availble in '%v' mode\n", cmd.Name, client.Shell.Mode)
		}
	} else {
		fmt.Printf("Unknown command: %s\n", parsed.Command)
	}
}

func buildCommandMap() map[string]types.Command {
	cmds := map[string]types.Command{}

	//TODO: Bad idea?
	register := func(c types.Command) {
		cmds[c.Name] = c
	}

	register(types.Command{
		Name: "/testCmd",
		Desc: "Test command Map builder",
		CmdFn: func(args []string, client *types.ClientInfo) {
			//TODO: Bad idea to put all the command logic in here?
			fmt.Println("Building the command map")
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/pollServer",
		Desc: "Get a list of active users on a server",
		CmdFn: func(args []string, client *types.ClientInfo) {
			sInfo := PollServer(client)
			fmt.Printf("Server Info\n Name: %s\n ID: %s\n", sInfo.ServerName, sInfo.ServerId)
			fmt.Println("Online Users:")
			for i, u := range sInfo.Users {
				fmt.Printf("[%v] %s: %s", i+1, u.UserId[:4], u.Username)
			}
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/addfriend",
		Desc: "Send a friend request",
		CmdFn: func(args []string, client *types.ClientInfo) {
			//TODO: Refactor out the need to pass in a reader
			todoReader := bufio.NewReader(os.Stdin)
			shellAddFriend(todoReader, client)
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/friends",
		Desc: "Display friends list",
		CmdFn: func(args []string, client *types.ClientInfo) {
			FriendList(client)
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/chat",
		Desc: "Chat with a friend",
		CmdFn: func(args []string, client *types.ClientInfo) {
			if len(args) == 0 {
				fmt.Println("Useage: /chat <friends username>")
				return
			}

			//TODO: Centralize state?
			client.Shell.Mode = types.ModeChat
			// client.Cache.CurrentChat
			fmt.Printf("Chat with %s\n", args[0])
			enterChat(client, args[0])

		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/exit",
		Desc: "Exit mshell",
		CmdFn: func(args []string, client *types.ClientInfo) {
			switch client.Shell.Mode {
			case types.ModeChat:
				fmt.Printf("Exiting chat with: %s\n", client.Cache.CurrentChat.User)
				client.Cache.CurrentChat = types.ChatDetails{}
				client.Shell.Mode = types.ModeDefault
			case types.ModeDefault:
				fmt.Println("Exiting mshell")
				os.Exit(0)
			}
		},
		Scope: []types.ShellMode{types.ModeDefault, types.ModeChat},
	})

	register(types.Command{
		Name: "/help",
		Desc: "List all available commands",
		CmdFn: func(args []string, client *types.ClientInfo) {
			fmt.Println("Available Commands:")
			for _, cmd := range cmds {
				fmt.Printf("%s: %s\n", cmd.Name, cmd.Desc)
			}
		},
		Scope: []types.ShellMode{types.ModeDefault, types.ModeChat},
	})

	return cmds
}

func MShell(client *types.ClientInfo) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go func() {
		err := ConnectPayloadStream(ctx, client)
		if err != nil {
			log.Fatalf("failed to connect message stream: %v\n", err)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	commands := buildCommandMap()

	for {
		printPrompt(client)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		parsed := inputParse(input)

		if parsed.IsCommand {
			dispatchCommand(commands, parsed, client)
		} else {
			switch client.Shell.Mode {
			case types.ModeChat:
				//TODO: active chat
				if err := SendMessage(client, input); err != nil {
					fmt.Printf("Send failed: %v\n", err)
					continue
				}
			default:
				// fmt.Println("Chat not engaged. Use <CHATCOMMAND> [username] to begin")
			}
		}
	}
}

func enterChat(c *types.ClientInfo, target string) {

	u := types.User{}

	//Useful?
	var created time.Time
	row := c.Pstatements.GetUser.QueryRowContext(context.TODO(), target)
	err := row.Scan(&u.Id, &u.Name, &u.Enckey, &u.Sigkey, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("Friend: %s, not found", target)
		}
		log.Fatalf("an error occured: %v", err)
	}

	sharedSecret, err := network.ComputeSharedSecret(c.Keys["EncryptionPrivateKey"], u.Enckey)
	if err != nil {
		// TODO: Error return
		log.Print("failed to compute shared secret")
		return
	}

	encode, hmac, err := crypto.DeriveKeys(c, sharedSecret)
	if err != nil {
		log.Fatalf("Failed to derive keys")
	}

	cd := types.ChatDetails{
		User:         u,
		SharedSecret: sharedSecret,
		EncKey:       encode,
		HmacKey:      hmac,
	}

	fmt.Printf("Loading messages with: %s", cd.User.Name)
	// loadMessages()

	c.Cache.CurrentChat = cd

}

func shellFriendRequests(ctx context.Context, c *types.ClientInfo) {
	if len(c.Cache.FriendRequests) == 0 {
		fmt.Println("No pending Friend requests :^[")
		return
	}

	fmt.Println("Pending Friend requests")
	inputReader := bufio.NewReader(os.Stdin)

	for k, fr := range c.Cache.FriendRequests {
		fmt.Printf("[%s] %s\n", fr.UserInfo.SigningPublicKey[:6], fr.UserInfo.Username)
		fmt.Printf(" y[Accept] / n[Decline] :")

		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(strings.ToLower(input))
		accepted := input == "y"

		if err := FriendResponse(ctx, c, c.Cache.FriendRequests[k], accepted); err != nil {
			log.Printf("Failed to decline invite: %v", err)
		}

	}
}

func FriendList(c *types.ClientInfo) {
	fmt.Println("Friend list:")
	friends, err := loadFriends(c)
	if err != nil {
		log.Fatal("Failed to load friends")
	}

	reader := bufio.NewReader(os.Stdin)

	if len(friends) == 0 {
		//TODO: Handle query loop here?
		fmt.Println("No friends yet.")

		if len(c.Cache.FriendRequests) != 0 {
			fmt.Print("See friend requests?: [y/n]")
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Printf("Error reading input: %v\n", err)
				return
			}

			input = strings.TrimSpace(strings.ToLower(input))
			accepted := input == "y"
			if accepted {
				shellFriendRequests(context.TODO(), c)
				return
			}
		}
		return
	}

	//TODO: add active status
	for _, f := range friends {
		fmt.Printf("[%s] %s\n", f.UserId, f.Username)
	}

	//TODO: DRY
	fmt.Print("See friend requests?: [y/n]")
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v\n", err)
		return
	}

	input = strings.TrimSpace(strings.ToLower(input))
	accepted := input == "y"
	if accepted {
		shellFriendRequests(context.TODO(), c)
		return
	}

}

func shellAddFriend(inputReader *bufio.Reader, c *types.ClientInfo) {
	fmt.Println("Online Users:")

	au := GetActiveUsers(c, &pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	})

	userList := make([]*pb.UserInfo, 0, len(au.Users))
	index := 0

	for _, user := range au.Users {
		if user.UserId == c.UserID.String() {
			continue
		} else {
			fmt.Printf("%d: %s\n", index+1, user.Username)
			userList = append(userList, user)
			index++
		}
	}

	fmt.Print("Enter the number of the user you want to invite (Enter to cancel): ")
	selectedIndexString, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v\n", err)
		return
	}

	selectedIndexString = strings.TrimSpace(selectedIndexString)
	if selectedIndexString == "" {
		fmt.Println("No user selected.")
		return
	}

	selectedIndex, err := strconv.Atoi(selectedIndexString)
	if err != nil || selectedIndex < 1 || selectedIndex > len(userList) {
		fmt.Println("Invalid selection. Please enter a valid user number.")
		return
	}

	selectedUser := userList[selectedIndex-1]

	err = FriendRequest(context.TODO(), c, selectedUser.UserId)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}

}

func loadChats(c *types.ClientInfo) error {
	rows, err := c.Pstatements.GetChats.QueryContext(context.TODO())
	if err != nil {
		return fmt.Errorf("error querying chats: %v", err)
	}

	defer func() {
		if rowErr := rows.Close(); rowErr != nil {
			log.Fatalf("error getting rows: %v\n", rowErr)
		}
	}()

	for rows.Next() {
		var (
			chat_id      uuid.UUID
			chat_name    string
			initiator    uuid.UUID
			participants []uuid.UUID
			stateStr     string
		)

		if err := rows.Scan(&chat_id, &chat_name, &initiator, &participants, &stateStr); err != nil {
			log.Printf("error scanning row: %v", err)
			return err
		}

		stateEnum, ok := pb.Chat_State_value[stateStr]
		if !ok {
			return fmt.Errorf("invalid chat state: %s", stateStr)
		}

		var participantsStrung []string
		for _, uID := range participants {
			participantsStrung = append(participantsStrung, uID.String())
		}

		chat := &pb.Chat{
			Id:           chat_id.String(),
			Name:         chat_name,
			State:        pb.Chat_State(stateEnum),
			Participants: participantsStrung,
		}

		c.Cache.Chats[chat_id] = chat
	}

	return nil
}

func loadMessages(c *types.ClientInfo) ([]types.MessageStruct, error) {
	rows, err := c.Pstatements.GetMessages.QueryContext(context.TODO(), c.Cache.CurrentChat)
	if err != nil {

		return nil, fmt.Errorf("error querying messages: %v", err)
	}

	defer func() {
		if rowErr := rows.Close(); rowErr != nil {
			log.Fatalf("error getting rows: %v\n", rowErr)
		}
	}()

	var messages []types.MessageStruct

	for rows.Next() {
		var msg types.MessageStruct
		if err := rows.Scan(&msg.Id, &msg.ChatId, &msg.Sender, &msg.Receiver, &msg.Direction, &msg.Content, &msg.Timestamp); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// TODO: Generic loading function?
func loadFriends(c *types.ClientInfo) ([]*pb.UserInfo, error) {
	rows, err := c.Pstatements.GetFriends.QueryContext(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error querying friends: %v", err)
	}

	defer func() {
		if rowErr := rows.Close(); rowErr != nil {
			log.Fatalf("error getting rows: %v\n", rowErr)
		}
	}()

	var users []*pb.UserInfo
	found := false

	//TODO: Clean this up
	friendsStr := struct {
		uInfo pb.UserInfo
		crAt  time.Time
	}{}

	for rows.Next() {
		// usr := &pb.UserInfo{}
		found = true
		if err := rows.Scan(&friendsStr.uInfo.UserId, &friendsStr.uInfo.Username, &friendsStr.uInfo.EncryptionPublicKey, &friendsStr.uInfo.SigningPublicKey, &friendsStr.crAt); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}
		users = append(users, &friendsStr.uInfo)
	}

	if !found {
		fmt.Println("No friends found.")
		return []*pb.UserInfo{}, nil
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

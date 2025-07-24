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
			err := cmd.CmdFn(parsed.Args, client)
			if err != nil {
				fmt.Printf("failed to dispatch command: %v\n", err)
				return
			}
		} else {
			fmt.Printf("'%s' command not availble in '%v' mode\n", cmd.Name, client.Shell.Mode)
		}
	} else {
		fmt.Printf("Unknown command: %s\n", parsed.Command)
	}
}

func buildCommandMap() (map[string]types.Command, error) {
	cmds := map[string]types.Command{}

	//TODO: Bad idea?
	register := func(c types.Command) {
		cmds[c.Name] = c
	}

	register(types.Command{
		Name: "/testCmd",
		Desc: "Test command Map builder",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			//TODO: Bad idea to put all the command logic in here?
			fmt.Println("Building the command map")
			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/pollServer",
		Desc: "Get a list of active users on a server",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			sInfo, err := PollServer(client)
			if err != nil {
				log.Println("failed to poll server")
				return err
			}
			fmt.Printf("Server Info\n Name: %s\n ID: %s\n", sInfo.ServerName, sInfo.ServerId)
			fmt.Println("Online Users:")
			for i, u := range sInfo.Users {
				fmt.Printf("[%v] %s: %s", i+1, u.UserId[:4], u.Username)
			}
			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/addfriend",
		Desc: "Send a friend request",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			//TODO: Refactor out the need to pass in a reader
			todoReader := bufio.NewReader(os.Stdin)
			err := shellAddFriend(todoReader, client)
			if err != nil {
				fmt.Printf("error executing addFriend shell: %v\n", err)
				return err
			}
			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/friends",
		Desc: "Display friends list",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			err := FriendList(client)
			if err != nil {
				fmt.Printf("error executing FriendList shell: %v\n", err)
				return err
			}
			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/chat",
		Desc: "Chat with a friend",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			if len(args) == 0 {
				fmt.Println("Useage: /chat <friends username>")
				return nil
			}
			//TODO: Centralize state?
			client.Shell.Mode = types.ModeChat
			// client.Cache.CurrentChat
			fmt.Printf("Chat with %s\n", args[0])
			err := enterChat(client, args[0])
			if err != nil {
				fmt.Printf("failed to enter chat: %v", err)
				return err
			}
			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault},
	})

	register(types.Command{
		Name: "/exit",
		Desc: "Exit mshell",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			switch client.Shell.Mode {
			case types.ModeChat:
				client.Cache.CurrentChat = types.ChatDetails{}
				client.Shell.Mode = types.ModeDefault
			case types.ModeDefault:
				fmt.Println("Exiting mshell")
				os.Exit(0)
			}

			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault, types.ModeChat},
	})

	register(types.Command{
		Name: "/help",
		Desc: "List all available commands",
		CmdFn: func(args []string, client *types.ClientInfo) error {
			fmt.Println("Available Commands:")
			for _, cmd := range cmds {
				fmt.Printf("%s: %s\n", cmd.Name, cmd.Desc)
			}
			return nil
		},
		Scope: []types.ShellMode{types.ModeDefault, types.ModeChat},
	})

	return cmds, nil
}

func MShell(client *types.ClientInfo) error {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go func() error {

		err := ConnectPayloadStream(ctx, client)
		if err != nil {
			fmt.Printf("Payload steam failure: %s\n", err)
			return err
		}

		return nil
	}()

	reader := bufio.NewReader(os.Stdin)
	commands, err := buildCommandMap()
	if err != nil {
		return fmt.Errorf("failed to build command map: %v", err)
	}

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
				if parsed.Raw == "" {
					continue
				}
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

func enterChat(c *types.ClientInfo, target string) error {

	u := types.User{}

	//Useful?
	var created time.Time
	row := c.Pstatements.GetUser.QueryRowContext(context.TODO(), target)
	err := row.Scan(&u.Id, &u.Name, &u.Enckey, &u.Sigkey, &u.KeyEx, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("Friend: %s, not found", target)
		}
		return fmt.Errorf("an error occured: %v", err)
	}

	sharedSecret, err := network.ComputeSharedSecret(c.Keys["EncryptionPrivateKey"], u.Enckey)
	if err != nil {
		log.Print("failed to compute shared secret")
		return err
	}

	encode, hmac, err := crypto.DeriveKeys(c, sharedSecret)
	if err != nil {
		fmt.Println("Failed to derive keys")
		return err
	}

	cd := types.ChatDetails{
		User:         u,
		SharedSecret: sharedSecret,
		EncKey:       encode,
		HmacKey:      hmac,
	}

	c.Cache.CurrentChat = cd

	msgs, err := loadMessages(c)
	if err != nil {
		fmt.Println("failure loading messages")
		return err
	}

	for _, v := range msgs {
		if v.Direction == "inbound" {
			fmt.Printf("[%s]: %s", c.Cache.CurrentChat.User.Name, v.Content)
		} else {
			fmt.Printf("[%s]: %s", c.Username, v.Content)
		}
	}

	return nil
}

func shellFriendRequests(ctx context.Context, c *types.ClientInfo) error {
	frs, err := loadFriendRequests(c)
	if err != nil {
		fmt.Printf("failed to load friend requests: %v\n", err)
	}

	if len(frs) == 0 {
		fmt.Println("No pending Friend requests :^[")
		return nil
	}

	fmt.Println("Pending Friend requests")
	inputReader := bufio.NewReader(os.Stdin)

	for _, fr := range frs {
		fmt.Printf("[%s] %s\n", fr.FriendId, fr.Username)
		fmt.Printf(" y[Accept] / n[Decline] :")

		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(strings.ToLower(input))
		accepted := input == "y"

    //TODO: This is broken, sending an incorrect friend request into FriendResponse
		pbfr := pb.FriendRequest{
			Target: fr.FriendId.String(),
			UserInfo: &pb.UserInfo{
				UserId:              c.UserID.String(),
				Username:            c.Username,
				EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
				SigningPublicKey:    c.Keys["SigningPublicKey"],
			},
		}

		if err := FriendResponse(ctx, c, &pbfr, accepted); err != nil {
			return fmt.Errorf("friend response failure: %v", err)
		}
	}

	return nil
}

func FriendList(c *types.ClientInfo) error {
	fmt.Println("Friend list:")
	friends, err := loadFriends(c)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	if len(friends) == 0 {
		//TODO: Handle query loop here?
		fmt.Println("No friends yet.")

		fmt.Print("See friend requests?: [y/n]")
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading input")
			return err
		}

		input = strings.TrimSpace(strings.ToLower(input))
		accepted := input == "y"
		if accepted {
			shellFriendRequests(context.TODO(), c)
			return nil
		}

		return nil
	}

	//TODO: add active status
	for _, f := range friends {
		fmt.Printf("[%s] %s\n", f.Id, f.Name)
	}

	//TODO: DRY
	fmt.Print("See friend requests?: [y/n]")
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Println("Error reading input")
		return err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	accepted := input == "y"
	if accepted {
		shellFriendRequests(context.TODO(), c)
		return nil
	}

	return nil
}

func shellAddFriend(inputReader *bufio.Reader, c *types.ClientInfo) error {
	fmt.Println("Online Users:")

	au, err := GetActiveUsers(c, &pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	})
	if err != nil {
		log.Println("failed to get active users")
		return err
	}

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
		return err
	}

	selectedIndexString = strings.TrimSpace(selectedIndexString)
	if selectedIndexString == "" {
		fmt.Println("No user selected.")
		return nil
	}

	selectedIndex, err := strconv.Atoi(selectedIndexString)
	if err != nil || selectedIndex < 1 || selectedIndex > len(userList) {
		fmt.Println("Invalid selection. Please enter a valid user number.")
		return nil
	}

	selectedUser := userList[selectedIndex-1]

	err = FriendRequest(context.TODO(), c, selectedUser)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}

	return nil
}

// TODO: Need to figure out the best way to display these
func loadMessages(c *types.ClientInfo) ([]types.MessageStruct, error) {
	rows, err := c.Pstatements.GetMessages.QueryContext(context.TODO(), c.Cache.CurrentChat.User.Id)
	if err != nil {

		return nil, fmt.Errorf("error querying messages: %v", err)
	}

	defer func() error {
		if rowErr := rows.Close(); rowErr != nil {
			fmt.Printf("error getting rows: %v\n", rowErr)
			return err
		}
		return nil
	}()

	var messages []types.MessageStruct

	for rows.Next() {
		var msg types.MessageStruct
		if err := rows.Scan(&msg.Id, &msg.FriendId, &msg.Direction, &msg.Content, &msg.Timestamp); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		decrypted, err := crypto.Decrypt(c, msg.Content)
		if err != nil {
			fmt.Printf("Failed to decrypt sealed message")
			return nil, err
		}

		msg.Content = decrypted

		messages = append(messages, msg)
	}

	return messages, nil
}

// TODO: Generic loading function?
func loadFriends(c *types.ClientInfo) ([]*types.User, error) {
	rows, err := c.Pstatements.GetFriends.QueryContext(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error querying friends: %v", err)
	}

	defer func() error {
		if rowErr := rows.Close(); rowErr != nil {
			fmt.Printf("error getting rows: %v\n", rowErr)
			return rowErr
		}

		return nil
	}()

	var users []*types.User
	found := false

	//TODO: Clean this up
	friendsStr := struct {
		uInfo types.User
		crAt  time.Time
	}{}

	for rows.Next() {
		// usr := &pb.UserInfo{}
		found = true
		if err := rows.Scan(&friendsStr.uInfo.Id, &friendsStr.uInfo.Name, &friendsStr.uInfo.Enckey, &friendsStr.uInfo.Sigkey, &friendsStr.uInfo.KeyEx, &friendsStr.crAt); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}
		users = append(users, &friendsStr.uInfo)
	}

	if !found {
		return []*types.User{}, nil
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// TODO: DRY?
func loadFriendRequests(c *types.ClientInfo) ([]*types.FriendRequest, error) {
	rows, err := c.Pstatements.GetFriendRequests.QueryContext(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error querying friend requests: %v", err)
	}

	defer func() error {
		if rowErr := rows.Close(); rowErr != nil {
			fmt.Printf("error getting rows: %v\n", rowErr)
			return rowErr
		}

		return nil
	}()

	var friendRequests []*types.FriendRequest
	var fr types.FriendRequest
	found := false

	for rows.Next() {
		// usr := &pb.UserInfo{}
		found = true
		if err := rows.Scan(&fr.FriendId, &fr.Username, &fr.Direction); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}

		if fr.Direction == "outbound" {
			continue
		} else {
			friendRequests = append(friendRequests, &fr)
		}
	}

	if !found {
		return []*types.FriendRequest{}, nil
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return friendRequests, nil
}

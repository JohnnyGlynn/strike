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

	"github.com/JohnnyGlynn/strike/internal/crypto"
	"github.com/JohnnyGlynn/strike/internal/network"
	"github.com/JohnnyGlynn/strike/internal/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

func MessagingShell(c *types.ClientInfo) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get messages
	// TODO: Pass a single client with everything we need
	go func() {
		err := ConnectPayloadStream(ctx, c)
		if err != nil {
			log.Fatalf("failed to connect message stream: %v\n", err)
		}
	}()

	inputReader := bufio.NewReader(os.Stdin)
	fmt.Println("/help for available commands")
	fmt.Printf("Enter chatTarget:message to send a message (e.g., '%v:HelloWorld') - Chat selection required\n", c.Username)

	commands := map[string]func(){
		"/addf": func() { shellAddFriend(inputReader, c) },
		//TODO: manage context
		"/fr": func() { shellFriendRequests(context.TODO(), c) },
		"/fl": func() { shellFriendList(c) },
		//"chat"
		"/beginchat": func() { shellBeginChat(c, inputReader) },
		"/chats":     func() { shellChat(inputReader, c) },
		"/invites":   func() { shellInvites(ctx, c) },
		"/help":      printHelp,
		"/exit": func() {
			cancel()
			fmt.Println("Exiting msgshell...")
			os.Exit(0) // Ensures we exit cleanly
		},
	}

	for {
		// Prompt for input
		//TODO: Retrieve the messages one time and cache
		if c.Cache.ActiveChat.Chat == nil {
			fmt.Print("[NO-CHAT]msgshell> ")
		} else {
			messages, err := loadMessages(c)
			if err != nil {
				log.Fatal("failed to load messages")
			}

			for _, msg := range messages {
				switch msg.Direction {
				case "outbound":
					fmt.Printf("You: %s\n", msg.Content)
				case "inbound":
					fmt.Printf("%s: %s\n", msg.Sender, msg.Content)
				default:
					fmt.Printf("Unknown: %s\n", msg.Content)
				}
			}

			fmt.Printf("[CHAT:%s]\n[%s]>", c.Cache.ActiveChat.Chat.Name[:20], c.Username)
		}

		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)

		if strings.HasPrefix(input, "/") {
			if cmd, ok := commands[input]; ok {
				cmd()
			} else {
				fmt.Printf("Unknown command: %s\n", input)
			}
			continue
		}

		if err := shellSendMessage(input, c); err != nil {
			fmt.Println(err)
			continue
		}
	}
}

func shellInvites(ctx context.Context, c *types.ClientInfo) {
	if len(c.Cache.Invites) == 0 {
		fmt.Println("No pending invites :^[")
		return
	}

	fmt.Println("Pending Invites")
	inputReader := bufio.NewReader(os.Stdin)

	for inviteID, chatRequest := range c.Cache.Invites {
		fmt.Printf("%v: %s [FROM: %s]\n", inviteID, chatRequest.Chat.Name, chatRequest.Initiator)
		fmt.Printf("y[Accept]/n[Decline]")

		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(strings.ToLower(input))
		accepted := input == "y"

		if err := ConfirmChat(ctx, c, chatRequest, accepted); err != nil {
			log.Printf("Failed to decline invite: %v", err)
		}
	}
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

func shellFriendList(c *types.ClientInfo) {
	fmt.Println("Friends")
	friends, err := loadFriends(c)
	if err != nil {
		log.Fatal("Failed to load friends")
	}

	//TODO: add active status
	for _, f := range friends {
		fmt.Printf("[%s] %s\n", f.UserId, f.Username)
	}
}

func shellChat(inputReader *bufio.Reader, c *types.ClientInfo) {
	if len(c.Cache.Chats) == 0 {
		if err := loadChats(c); err != nil {
			log.Printf("Error loading chats: %v", err)
			return
		}
		if len(c.Cache.Chats) == 0 {
			fmt.Println("No chats available")
			return
		}
	}

	fmt.Println("Available Chats:")

	chatList := make([]*pb.Chat, 0, len(c.Cache.Chats))
	index := 1

	for _, chat := range c.Cache.Chats {
		fmt.Printf("%d: %s [STATE: %v]\n", index, chat.Name, chat.State.String())
		chatList = append(chatList, chat)
		index++
	}

	fmt.Print("Enter the chat number to set active (Enter to cancel): ")
	selectedIndexString, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v\n", err)
		return
	}

	selectedIndexString = strings.TrimSpace(selectedIndexString)
	if selectedIndexString == "" {
		fmt.Println("No chat selected.")
		return
	}

	selectedIndex, err := strconv.Atoi(selectedIndexString)
	if err != nil || selectedIndex < 1 || selectedIndex > len(chatList) {
		fmt.Println("Invalid selection. Please enter a valid chat number.")
		return
	}

	selectedChat := chatList[selectedIndex-1]

	if c.Cache.ActiveChat.Chat == selectedChat {
		fmt.Printf("%s already active", selectedChat.Name)
		return
	}

	c.Cache.ActiveChat.Chat = selectedChat
	fmt.Printf("Active chat: %s\n", c.Cache.ActiveChat.Chat.Name)

	participants := c.Cache.ActiveChat.Chat.Participants

	for k, v := range participants {
		if v == c.UserID.String() {
			participants = slices.Delete(participants, k, k+1)
			break
		}
	}

	if len(participants) == 0 {
		log.Print("No other participants in the chat")
		return
	}

	var targetUser string
	row := c.Pstatements.GetUsername.QueryRowContext(context.TODO(), participants[0])
	err = row.Scan(&targetUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Fatalf("DB error: %v", err)
		}
		log.Fatalf("an error occured: %v", err)
	}

	uinfo := &pb.UserInfo{
		Username: targetUser,
	}

	target, err := c.Pbclient.UserRequest(context.TODO(), uinfo)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}

	sharedSecret, err := network.ComputeSharedSecret(c.Keys["EncryptionPrivateKey"], target.EncryptionPublicKey)
	if err != nil {
		// TODO: Error return
		log.Print("failed to compute shared secret")
		return
	}

	c.Cache.ActiveChat.SharedSecret = sharedSecret

	err = crypto.DeriveKeys(c, sharedSecret)
	if err != nil {
		log.Fatalf("Failed to derive keys")
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
			fmt.Printf("%d: %s\n", index, user.Username)
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

func shellBeginChat(c *types.ClientInfo, inputReader *bufio.Reader) {
	fmt.Print("Invite User> ")
	inviteUser, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading invite input user: %v\n", err)
		return
	}
	inviteUser = strings.TrimSpace(inviteUser)

	var targetUser *pb.UserInfo

	au := GetActiveUsers(c, &pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	})

	//TODO Add user function directly
	for _, value := range au.Users {
		if value.Username == inviteUser {
			targetUser = value
		}

	}

	fmt.Print("Chat Name> ")
	chatName, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading invite input chat name: %v\n", err)
		return
	}
	chatName = strings.TrimSpace(chatName)

	err = BeginChat(c, uuid.MustParse(targetUser.UserId), chatName)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}
}

func shellSendMessage(input string, c *types.ClientInfo) error {
	if input == "" {
		return nil
	}

	userAndMessage := strings.SplitN(input, ":", 2) // Check for : then splint into target and message
	if len(userAndMessage) != 2 {
		return fmt.Errorf("invalid format. Use recipient:message")
	}

	// TODO: Stopgap handle this elsewhere
	if c.Cache.ActiveChat.Chat == nil {
		return fmt.Errorf("no chat has been selected. use /chats to enable a chat first")
	}

	target, message := userAndMessage[0], userAndMessage[1]

	// TODO: Migrate messaging shell to active chat only, stop having to query uuid on every message
	var targetID uuid.UUID
	row := c.Pstatements.GetUserId.QueryRowContext(context.TODO(), target)
	err := row.Scan(&targetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Fatalf("DB error: %v", err)
		}
		log.Fatalf("an error occured: %v", err)
	}

	SendMessage(c, targetID, message)

	return nil
}

func printHelp() {
	fmt.Println("---Available Commands---")
	fmt.Println("/addf:       Send friend request by username")
	fmt.Println("/fr:         View and respond to friend requests")
	fmt.Println("/fl:         List all friends")

	fmt.Println("/beginchat:  Invite a user to a chat")
	fmt.Println("/chats:      List joined chats and set one active")
	fmt.Println("/invites:    See and respond to chat invites")
	fmt.Println("/help:       ...")
	fmt.Println("/exit:       ...")
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
	rows, err := c.Pstatements.GetMessages.QueryContext(context.TODO(), c.Cache.ActiveChat)
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

	for rows.Next() {
		usr := &pb.UserInfo{}
		if err := rows.Scan(&usr.UserId, &usr.Username, &usr.EncryptionPublicKey, &usr.SigningPublicKey); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}
		users = append(users, usr)
	}

	return users, nil
}

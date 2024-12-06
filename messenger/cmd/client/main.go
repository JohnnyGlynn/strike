package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	pb "messenger/proto"

	_ "github.com/mattn/go-sqlite3"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Helper function to get user input
func getInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func ensureDirExists(dirPath string) error {
	// Expand `~` to the user's home directory
	dirPath = os.ExpandEnv(filepath.Clean(dirPath))

	// Check if the directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		// Create the directory including parents
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}
	}
	return nil
}

func generateKeyPair(dirPath string) (string, string, error) {

	if err := ensureDirExists(dirPath); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Directory ensured:", dirPath)
	}
	// Define file paths
	privKeyPath := filepath.Join(dirPath, "private_key.pem")
	pubKeyPath := filepath.Join(dirPath, "public_key.pem")

	var privKeyPEM, pubKeyPEM string

	// Check for private key
	if _, err := os.Stat(privKeyPath); os.IsNotExist(err) {
		// Generate private key
		privKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", "", fmt.Errorf("failed to generate private key: %w", err)
		}

		// Encode private key to PEM format
		privKeyBlock := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
		}
		privKeyPEM = string(pem.EncodeToMemory(privKeyBlock))

		// Write private key to file
		if err := os.WriteFile(privKeyPath, []byte(privKeyPEM), 0600); err != nil {
			return "", "", fmt.Errorf("failed to write private key to file: %w", err)
		}
		fmt.Println("Private key generated and saved:", privKeyPath)
	} else {
		// Read private key if it exists
		data, err := os.ReadFile(privKeyPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read private key: %w", err)
		}
		privKeyPEM = string(data)
	}

	// Check for public key
	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		// Decode private key to extract public key
		block, _ := pem.Decode([]byte(privKeyPEM))
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return "", "", fmt.Errorf("invalid private key PEM format")
		}
		privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse private key: %w", err)
		}

		// Encode public key to PEM format
		pubKeyBlock := &pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(&privKey.PublicKey),
		}
		pubKeyPEM = string(pem.EncodeToMemory(pubKeyBlock))

		// Write public key to file
		if err := os.WriteFile(pubKeyPath, []byte(pubKeyPEM), 0644); err != nil {
			return "", "", fmt.Errorf("failed to write public key to file: %w", err)
		}
		fmt.Println("Public key generated and saved:", pubKeyPath)
	} else {
		// Read public key if it exists
		data, err := os.ReadFile(pubKeyPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read public key: %w", err)
		}
		pubKeyPEM = string(data)
	}

	return privKeyPEM, pubKeyPEM, nil
}

func encryptMessage(publicKeyPEM string, message string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		return "", fmt.Errorf("invalid public key")
	}

	pubKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %v", err)
	}

	encryptedBytes, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, []byte(message), nil)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt message: %v", err)
	}

	return base64.StdEncoding.EncodeToString(encryptedBytes), nil
}

func decryptMessage(privateKeyPEM string, encryptedMessage string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return "", fmt.Errorf("invalid private key")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}

	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedMessage)
	if err != nil {
		return "", fmt.Errorf("failed to decode message: %v", err)
	}

	decryptedBytes, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, encryptedBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message: %v", err)
	}

	return string(decryptedBytes), nil
}

func sendMessage(client pb.RouterServiceClient, sender string, db *sql.DB) {
	recipient := getInput("Enter recipient: ")
	body := getInput("Enter message body: ")

	// Fetch recipient's public key
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.GetPublicKey(ctx, &pb.PublicKeyRequest{Username: recipient})
	if err != nil {
		log.Printf("Failed to fetch recipient's public key: %v", err)
		return
	}

	// Encrypt the message
	encryptedBody, err := encryptMessage(resp.PublicKey, body)
	if err != nil {
		log.Printf("Failed to encrypt message: %v", err)
		return
	}

	// Send the encrypted message
	_, err = client.SendMessage(ctx, &pb.Message{
		Sender:    sender,
		Recipient: recipient,
		Body:      encryptedBody,
		Timestamp: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("Failed to send message: %v", err)
	} else {
		log.Println("Message sent successfully")
		saveSentMessage(recipient, body, db)
	}
}
func sendHeartbeats(client pb.RouterServiceClient, username string) {
	for {
		time.Sleep(30 * time.Second) // Send a heartbeat every 30 seconds

		_, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{Username: username})
		if err != nil {
			log.Printf("Heartbeat failed for %s: %v", username, err)
			return
		}

		// log.Printf("Heartbeat sent for %s", username)
	}
}

func getChatHistoryWithUser(db *sql.DB, user string) ([]string, error) {
	query := `SELECT sender, text, timestamp, direction FROM messages 
	          WHERE sender = ? OR recipient = ? 
	          ORDER BY timestamp DESC;`
	rows, err := db.Query(query, user, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []string
	for rows.Next() {
		var sender, text, timestamp, direction string
		if err := rows.Scan(&sender, &text, &timestamp, &direction); err != nil {
			return nil, err
		}
		formatted := fmt.Sprintf("[%s] (%s) %s: %s", timestamp, direction, sender, text)
		history = append(history, formatted)
	}
	return history, nil
}

func saveSentMessage(recipient, text string, db *sql.DB) {
	insertQuery := `INSERT INTO messages (sender, recipient, text, timestamp, direction) VALUES (?, ?, ?, ?, ?);`
	_, err := db.Exec(insertQuery, "me", recipient, text, time.Now().Format(time.RFC3339), "sent")
	if err != nil {
		log.Printf("Failed to save sent message: %v", err)
	}
}

func saveReceivedMessage(sender, text string, db *sql.DB) {
	insertQuery := `INSERT INTO messages (sender, recipient, text, timestamp, direction) VALUES (?, ?, ?, ?, ?);`
	_, err := db.Exec(insertQuery, sender, "me", text, time.Now().Format(time.RFC3339), "received")
	if err != nil {
		log.Printf("Failed to save received message: %v", err)
	}
}
func getChatParticipants(db *sql.DB) ([]string, error) {
	query := `
	SELECT DISTINCT 
	    CASE 
	        WHEN sender = 'me' THEN recipient 
	        ELSE sender 
	    END AS participant
	FROM messages;`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []string
	for rows.Next() {
		var participant string
		if err := rows.Scan(&participant); err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	return participants, nil
}

func selectChatParticipant(participants []string) (string, error) {
	fmt.Println("Chats:")
	for i, participant := range participants {
		fmt.Printf("%d. %s\n", i+1, participant)
	}

	fmt.Print("Select a chat (enter number): ")
	var choice int
	_, err := fmt.Scan(&choice)
	if err != nil || choice < 1 || choice > len(participants) {
		return "", fmt.Errorf("invalid selection")
	}

	return participants[choice-1], nil
}
func viewChat(db *sql.DB) {
	// Get list of participants
	participants, err := getChatParticipants(db)
	if err != nil {
		log.Printf("Failed to fetch participants: %v", err)
		return
	}

	if len(participants) == 0 {
		fmt.Println("No chats available.")
		return
	}

	// Let user select a participant
	participant, err := selectChatParticipant(participants)
	if err != nil {
		log.Printf("Invalid selection: %v", err)
		return
	}

	// Fetch and display chat history with the selected participant
	history, err := getChatHistoryWithUser(db, participant)
	if err != nil {
		log.Printf("Failed to fetch chat history: %v", err)
		return
	}

	fmt.Printf("Chat history with %s:\n", participant)
	for _, msg := range history {
		fmt.Println(msg)
	}
}

func initDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create the messages table if it doesn't exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT NOT NULL,
		recipient TEXT NOT NULL,
		text TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		direction TEXT NOT NULL
	);`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, err
	}

	return db, nil
}
func main() {

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	username := getInput("Enter your username: ")

	usr, err := user.Current()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	path := usr.HomeDir + "/.messenger/" + username + "/"

	privateKeyPEM, publicKeyPEM, err := generateKeyPair(path)
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Keys created successfully")
	}

	dbPath := path + "messages.db"

	msgdb, err := initDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer msgdb.Close()

	// conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	conn, err := grpc.NewClient("localhost:50051", opts...)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewRouterServiceClient(conn)

	// Register with the router
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = client.Register(ctx, &pb.RegisterRequest{
		Username:  username,
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	log.Println("Registered successfully")

	go sendHeartbeats(client, username)

	// Stream messages in a separate goroutine
	go func() {
		stream, err := client.ReceiveMessages(context.Background(), &pb.ReceiveRequest{
			Username: username,
		})
		if err != nil {
			log.Fatalf("Failed to receive messages: %v", err)
		}

		for {
			msg, err := stream.Recv()
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				break
			}
			// Decrypt the message
			message, err := decryptMessage(privateKeyPEM, msg.Body)
			if err != nil {
				log.Printf("Error decrypting message: %v", err)
				continue
			}
			log.Printf("Message from %s:\n %s\n%s", msg.Sender, string(message), msg.Timestamp)
			saveReceivedMessage(msg.Sender, string(message), msgdb)
		}
	}()

	for {
		fmt.Println("\nChat CLI Tool")
		fmt.Println("1. Send a message")
		fmt.Println("2. Chat History")
		fmt.Println("3. Exit")
		choice := getInput("Choose an option: ")

		switch choice {
		case "1":
			sendMessage(client, username, msgdb)
		case "2":
			viewChat(msgdb)
		case "3":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid option. Please try again.")
		}
	}
}

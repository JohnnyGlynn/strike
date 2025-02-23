package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type Demultiplexer struct {
	chatRequestChannel chan *pb.BeginChatRequest
	chatConfirmChannel chan *pb.ConfirmChatRequest
	envelopeChannel    chan *pb.Envelope
	keyExchangeChannel chan *pb.KeyExchangeRequest

	// Workers
	envelopeWorkerCount           int
	chatRequestWorkerCount        int
	chatConfirmWorkerCount        int
	keyExchangeRequestWorkerCount int

	// mutex for workers
	mu sync.Mutex
}

func NewDemultiplexer(c pb.StrikeClient, chatCache map[string]*pb.Chat, inviteCache map[string]*pb.BeginChatRequest, keys map[string][]byte, username string) *Demultiplexer {
	d := &Demultiplexer{
		chatRequestChannel: make(chan *pb.BeginChatRequest, 20),
		chatConfirmChannel: make(chan *pb.ConfirmChatRequest, 20),
		envelopeChannel:    make(chan *pb.Envelope, 200),
		keyExchangeChannel: make(chan *pb.KeyExchangeRequest, 20),
	}

	// Baseline workers
	d.mu.Lock()
	d.envelopeWorkerCount = 1
	d.chatRequestWorkerCount = 1
	d.chatConfirmWorkerCount = 1
	d.keyExchangeRequestWorkerCount = 1
	d.mu.Unlock()

	// Run demultiplexer channel processors - Permanent processors
	go ProcessEnvelopes(d.envelopeChannel, 0, &d.envelopeWorkerCount, &d.mu)
	go ProcessChatRequests(d.chatRequestChannel, inviteCache, 0, &d.chatRequestWorkerCount, &d.mu)
	go ProcessConfirmChatRequests(d.chatConfirmChannel, chatCache, 0, &d.chatConfirmWorkerCount, &d.mu)
	go ProcessKeyExchangeRequests(c, d.keyExchangeChannel, chatCache, 0, &d.keyExchangeRequestWorkerCount, &d.mu, keys, username)

	return d
}

func (d *Demultiplexer) Dispatcher(msg *pb.MessageStreamPayload) {
	switch payload := msg.Payload.(type) {
	case *pb.MessageStreamPayload_Envelope:
		select {
		case d.envelopeChannel <- payload.Envelope:
		default:
			log.Printf("WARNING: Channel full - Envelope dropped - Sender: %v\n", payload.Envelope.FromUser)
		}
	case *pb.MessageStreamPayload_ChatRequest:
		select {
		case d.chatRequestChannel <- payload.ChatRequest:
		default:
			log.Printf("WARNING: Channel full - Chat Request dropped - Sender: %v\n", payload.ChatRequest.Initiator)
		}
	case *pb.MessageStreamPayload_ChatConfirm:
		select {
		case d.chatConfirmChannel <- payload.ChatConfirm:
		default:
			log.Printf("WARNING: Channel full - Chat Confirm dropped - Sender: %v\n", payload.ChatConfirm.Confirmer)
			// TODO: Retry to create chats if this fails
		}
	case *pb.MessageStreamPayload_KeyExchRequest:
		select {
		case d.keyExchangeChannel <- payload.KeyExchRequest:
		default:
			log.Printf("WARNING: Channel full - Key exchange request dropped - Sender: %v\n", payload.KeyExchRequest)
		}
	default:
		log.Println("Unknown payload type")
	}
}

func (d *Demultiplexer) StartMonitoring(c pb.StrikeClient, chatCache map[string]*pb.Chat, inviteCache map[string]*pb.BeginChatRequest, keys map[string][]byte, username string) {
	const ephemeralTimeout = 5 * time.Second

	// Monitor our channels - spawn workers if needed - more for messages obviously
	go monitorChannel(d.envelopeChannel, 20, 5, &d.envelopeWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessEnvelopes(d.envelopeChannel, ephemeralTimeout, &d.envelopeWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.chatRequestChannel, 10, 3, &d.chatRequestWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessChatRequests(d.chatRequestChannel, inviteCache, ephemeralTimeout, &d.chatRequestWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.chatConfirmChannel, 10, 3, &d.chatConfirmWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessConfirmChatRequests(d.chatConfirmChannel, chatCache, ephemeralTimeout, &d.chatConfirmWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.keyExchangeChannel, 10, 3, &d.keyExchangeRequestWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessKeyExchangeRequests(c, d.keyExchangeChannel, chatCache, ephemeralTimeout, &d.keyExchangeRequestWorkerCount, &d.mu, keys, username) // TODO: This is a mes
		},
	)
}

func ProcessEnvelopes(ch <-chan *pb.Envelope, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case envelope, ok := <-ch:
			if !ok {
				return
			}
			fmt.Printf("[%s] [%s] [From:%s] : %s\n", envelope.SentAt.AsTime(), envelope.Chat.Name, envelope.FromUser, envelope.Message)
		case <-timeoutCh:
			fmt.Printf("Envelope worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessChatRequests(ch <-chan *pb.BeginChatRequest, cache map[string]*pb.BeginChatRequest, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case chatRequest, ok := <-ch:
			if !ok {
				return
			}
			fmt.Printf("Chat Invite recieved from:%v Chat Name: %v\n", chatRequest.Initiator, chatRequest.Chat.Name)
			// Recieve an invite, cache it
			cache[chatRequest.InviteId] = chatRequest
		case <-timeoutCh:
			fmt.Printf("ChatRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessConfirmChatRequests(ch <-chan *pb.ConfirmChatRequest, cache map[string]*pb.Chat, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case confirmation, ok := <-ch:
			if !ok {
				return
			}

			if confirmation.State {
				fmt.Printf("Invitation %v for:%s, With: %s, Status: Accepted\n", confirmation.InviteId, confirmation.Chat.Name, confirmation.Confirmer)
				cache[confirmation.Chat.Id] = confirmation.Chat
			} else {
				fmt.Printf("Invitation %v for:%s, With: %s, Status: Declined\n", confirmation.InviteId, confirmation.Chat.Name, confirmation.Confirmer)
			}
		case <-timeoutCh:
			fmt.Printf("ConfirmChatRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessKeyExchangeRequests(c pb.StrikeClient, ch <-chan *pb.KeyExchangeRequest, cache map[string]*pb.Chat, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex, keys map[string][]byte, username string) {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case keyExReq, ok := <-ch:
			if !ok {
				return
			}
			fmt.Printf("Key exchange initiated for: %v\n", keyExReq.ChatId)
			chat := cache[keyExReq.ChatId]

			sharedSecret, err := ComputeSharedSecret(keys["EncryptionPrivateKey"], keyExReq.CurvePublicKey)
			if err != nil {
				// TODO: Error return
				log.Print("failed to compute shared secret")
			}

			// DB OPERATIONS HERE
			fmt.Printf("INSERT INTO DB DONT PRINT THIS: %v", sharedSecret)

			// TODO: Signature is gross
			ReciprocateKeyExchange(context.TODO(), c, keyExReq.Target, username, keys["SigningPrivateKey"], keys["EncryptionPublicKey"], chat)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

// Generic Channel monitor- Provide it any channel and respective processor function
func monitorChannel[T any](ch <-chan T, threshold, maxWorkers int, workerCount *int, idleTimeout time.Duration, mu *sync.Mutex, spawnWorker func()) {
	ticker := time.NewTicker(5 * time.Second) // Check channel every 5 seconds
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		if len(ch) > threshold && *workerCount < maxWorkers {
			*workerCount++
			log.Printf("Spawning new ephemeral worker; current workers: %d", *workerCount)
			// Callback generic for any of the Process* functions
			go spawnWorker()
		}
		mu.Unlock()
	}
}

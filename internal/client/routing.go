package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

type Demultiplexer struct {
	chatRequestChannel             chan *pb.BeginChatRequest
	chatConfirmChannel             chan *pb.ConfirmChatRequest
	envelopeChannel                chan *pb.Envelope
	keyExchangeChannel             chan *pb.KeyExchangeRequest
	keyExchangeResponseChannel     chan *pb.KeyExchangeResponse
	keyExchangeConfirmationChannel chan *pb.KeyExchangeConfirmation

	// Workers
	envelopeWorkerCount           int
	chatRequestWorkerCount        int
	chatConfirmWorkerCount        int
	keyExchangeRequestWorkerCount int

	// mutex for workers
	// TODO: Global locking bad idea?
	mu sync.Mutex
}

func NewDemultiplexer(c ClientInfo) *Demultiplexer {
	d := &Demultiplexer{
		chatRequestChannel: make(chan *pb.BeginChatRequest, 20),
		chatConfirmChannel: make(chan *pb.ConfirmChatRequest, 20),
		envelopeChannel:    make(chan *pb.Envelope, 200),
		// TODO: There has to be a better way
		keyExchangeChannel:             make(chan *pb.KeyExchangeRequest, 20),
		keyExchangeResponseChannel:     make(chan *pb.KeyExchangeResponse, 20),
		keyExchangeConfirmationChannel: make(chan *pb.KeyExchangeConfirmation, 20),
	}

	// Baseline workers
	d.mu.Lock()
	d.envelopeWorkerCount = 1
	d.chatRequestWorkerCount = 1
	d.chatConfirmWorkerCount = 1
	d.keyExchangeRequestWorkerCount = 1
	d.mu.Unlock()

	// Run demultiplexer channel processors - Permanent processors
	go ProcessEnvelopes(d.envelopeChannel, c, 0, &d.envelopeWorkerCount, &d.mu)
	go ProcessChatRequests(d.chatRequestChannel, c, 0, &d.chatRequestWorkerCount, &d.mu)
	go ProcessConfirmChatRequests(c, d.chatConfirmChannel, 0, &d.chatConfirmWorkerCount, &d.mu)
	go ProcessKeyExchangeRequests(c, d.keyExchangeChannel, 0, &d.keyExchangeRequestWorkerCount, &d.mu)
	go ProcessKeyExchangeResponses(c, d.keyExchangeResponseChannel, 0, &d.keyExchangeRequestWorkerCount, &d.mu)
	go ProcessKeyExchangeConfirmations(c, d.keyExchangeConfirmationChannel, 0, &d.keyExchangeRequestWorkerCount, &d.mu)

	return d
}

func (d *Demultiplexer) Dispatcher(msg *pb.StreamPayload) {
	switch payload := msg.Payload.(type) {
	case *pb.StreamPayload_Envelope:
		select {
		case d.envelopeChannel <- payload.Envelope:
		default:
			log.Printf("WARNING: Channel full - Envelope dropped - Sender: %v\n", payload.Envelope.FromUser)
		}
	case *pb.StreamPayload_ChatRequest:
		select {
		case d.chatRequestChannel <- payload.ChatRequest:
		default:
			log.Printf("WARNING: Channel full - Chat Request dropped - Sender: %v\n", payload.ChatRequest.Initiator)
		}
	case *pb.StreamPayload_ChatConfirm:
		select {
		case d.chatConfirmChannel <- payload.ChatConfirm:
		default:
			log.Printf("WARNING: Channel full - Chat Confirm dropped - Sender: %v\n", payload.ChatConfirm.Confirmer)
			// TODO: Retry to create chats if this fails
		}
		// TODO: Do better than X was dropped
	case *pb.StreamPayload_KeyExchRequest:
		select {
		case d.keyExchangeChannel <- payload.KeyExchRequest:
		default:
			log.Printf("WARNING: Channel full - Key exchange request dropped - Sender: %v\n", payload.KeyExchRequest)
		}
	case *pb.StreamPayload_KeyExchResponse:
		select {
		case d.keyExchangeResponseChannel <- payload.KeyExchResponse:
		default:
			log.Printf("WARNING: Channel full - Key exchange response dropped - Sender: %v\n", payload.KeyExchResponse)
		}
	case *pb.StreamPayload_KeyExchConfirm:
		select {
		case d.keyExchangeConfirmationChannel <- payload.KeyExchConfirm:
		default:
			log.Printf("WARNING: Channel full - Key exchange confirmation dropped - Sender: %v\n", payload.KeyExchConfirm)
		}

	default:
		log.Println("Unknown payload type")
	}
}

func (d *Demultiplexer) StartMonitoring(c ClientInfo) {
	const ephemeralTimeout = 5 * time.Second

	// Monitor our channels - spawn workers if needed - more for messages obviously
	go monitorChannel(d.envelopeChannel, 20, 5, &d.envelopeWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessEnvelopes(d.envelopeChannel, c, ephemeralTimeout, &d.envelopeWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.chatRequestChannel, 10, 3, &d.chatRequestWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessChatRequests(d.chatRequestChannel, c, ephemeralTimeout, &d.chatRequestWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.chatConfirmChannel, 10, 3, &d.chatConfirmWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessConfirmChatRequests(c, d.chatConfirmChannel, ephemeralTimeout, &d.chatConfirmWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.keyExchangeChannel, 10, 3, &d.keyExchangeRequestWorkerCount, ephemeralTimeout, &d.mu,
		func() {
			ProcessKeyExchangeRequests(c, d.keyExchangeChannel, ephemeralTimeout, &d.keyExchangeRequestWorkerCount, &d.mu) // TODO: This is a mes
		},
	)
}

func ProcessEnvelopes(ch <-chan *pb.Envelope, c ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
			_, err := c.DBpool.Exec(context.TODO(), c.Pstatements.SaveMessage, uuid.New(), envelope.Chat.Id, envelope.FromUser, envelope.Message)
			if err != nil {
				log.Fatalf("Failed to save message")
			}
		case <-timeoutCh:
			fmt.Printf("Envelope worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessChatRequests(ch <-chan *pb.BeginChatRequest, c ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
			c.Cache.Invites[chatRequest.InviteId] = chatRequest
		case <-timeoutCh:
			fmt.Printf("ChatRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessConfirmChatRequests(c ClientInfo, ch <-chan *pb.ConfirmChatRequest, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
				c.Cache.Chats[confirmation.Chat.Id] = confirmation.Chat
				// TODO: Initiator will probably have to change
				InitiateKeyExchange(context.TODO(), c.Pbclient, confirmation.Confirmer, c.UserID, c.Keys["SigningPrivateKey"], c.Keys["EncryptionPublicKey"], confirmation.Chat)
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

func ProcessKeyExchangeRequests(c ClientInfo, ch <-chan *pb.KeyExchangeRequest, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
			chat, exists := c.Cache.Chats[keyExReq.ChatId]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExReq.ChatId)
				return
			}

			sharedSecret, err := ComputeSharedSecret(c.Keys["EncryptionPrivateKey"], keyExReq.CurvePublicKey)
			if err != nil {
				// TODO: Error return
				log.Print("failed to compute shared secret")
        return
			}

			_, err = c.DBpool.Exec(context.TODO(), c.Pstatements.CreateChat, keyExReq.ChatId, chat.Name, keyExReq.SenderUserId, chat.Participants, pb.Chat_KEY_EXCHANGE_PENDING.String(), sharedSecret)
			if err != nil {
				log.Fatal("Failed to save Chat")
			}

			// TODO: Signature is gross
			ReciprocateKeyExchange(context.TODO(), c.Pbclient, keyExReq.Target, c.UserID, c.Keys["SigningPrivateKey"], c.Keys["EncryptionPublicKey"], chat)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessKeyExchangeResponses(c ClientInfo, ch <-chan *pb.KeyExchangeResponse, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case keyExRes, ok := <-ch:
			if !ok {
				return
			}
			fmt.Printf("Key exchange response for: %v\n", keyExRes.ChatId)
			chat, exists := c.Cache.Chats[keyExRes.ChatId]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExRes.ChatId)
				return
			}

			sharedSecret, err := ComputeSharedSecret(c.Keys["EncryptionPrivateKey"], keyExRes.CurvePublicKey)
			if err != nil {
				// TODO: Error return
				log.Print("failed to compute shared secret")
			}

			// As the map is *pb.chat it should update directly.
			// TODO: More robust cache rather than maps (Redis?)
			chat.State = pb.Chat_KEY_EXCHANGE_PENDING

			_, err = c.DBpool.Exec(context.TODO(), c.Pstatements.CreateChat, keyExRes.ChatId, chat.Name, keyExRes.ResponderUserId, chat.Participants, chat.State.String(), sharedSecret)
			if err != nil {
				log.Fatal("Failed to save Chat")
			}

			// TODO: Something fails so the confirmations can be false???
			ConfirmKeyExchange(context.TODO(), c.Pbclient, keyExRes.ResponderUserId, true, chat)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeResponse worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessKeyExchangeConfirmations(c ClientInfo, ch <-chan *pb.KeyExchangeConfirmation, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case keyExCon, ok := <-ch:
			if !ok {
				return
			}
			chat, exists := c.Cache.Chats[keyExCon.ChatId]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExCon.ChatId)
				return
			}

			if chat.State == pb.Chat_KEY_EXCHANGE_PENDING {
				fmt.Printf("Key exchange confirmation for: %v\n", keyExCon.ChatId)

				// TODO: More robust cache rather than maps (Redis?)
				chat.State = pb.Chat_ENCRYPTED

				_, err := c.DBpool.Exec(context.TODO(), c.Pstatements.UpdateChatState, pb.Chat_ENCRYPTED.String(), keyExCon.ChatId)
				if err != nil {
					log.Fatal("Failed to save Chat")
				}

				ConfirmKeyExchange(context.TODO(), c.Pbclient, keyExCon.ConfirmerUserId, true, chat)
			} else {
				// TODO: right approach to return?
				return
			}

		case <-timeoutCh:
			fmt.Printf("KeyExchangeResponse worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
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

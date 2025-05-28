package network

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/JohnnyGlynn/strike/internal/crypto"
	"github.com/JohnnyGlynn/strike/internal/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

type Demultiplexer struct {
	chatRequestChannel             chan *pb.BeginChatRequest
	chatConfirmChannel             chan *pb.ConfirmChatRequest
	encenvelopeChannel             chan *pb.EncryptedEnvelope
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

func NewDemultiplexer(c *types.ClientInfo) *Demultiplexer {
	d := &Demultiplexer{
		chatRequestChannel: make(chan *pb.BeginChatRequest, 20),
		chatConfirmChannel: make(chan *pb.ConfirmChatRequest, 20),
		encenvelopeChannel: make(chan *pb.EncryptedEnvelope, 200),
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
	go ProcessEnvelopes(d.encenvelopeChannel, c, 0, &d.envelopeWorkerCount, &d.mu)
	go ProcessChatRequests(d.chatRequestChannel, c, 0, &d.chatRequestWorkerCount, &d.mu)
	go ProcessConfirmChatRequests(c, d.chatConfirmChannel, 0, &d.chatConfirmWorkerCount, &d.mu)
	go ProcessKeyExchangeRequests(c, d.keyExchangeChannel, 0, &d.keyExchangeRequestWorkerCount, &d.mu)
	go ProcessKeyExchangeResponses(c, d.keyExchangeResponseChannel, 0, &d.keyExchangeRequestWorkerCount, &d.mu)
	go ProcessKeyExchangeConfirmations(c, d.keyExchangeConfirmationChannel, 0, &d.keyExchangeRequestWorkerCount, &d.mu)

	return d
}

func (d *Demultiplexer) Dispatcher(msg *pb.StreamPayload) {
	switch payload := msg.Payload.(type) {
	case *pb.StreamPayload_Encenv:
		select {
		case d.encenvelopeChannel <- payload.Encenv:
		default:
			log.Printf("WARNING: Channel full - Envelope dropped - Sender: %v\n", payload.Encenv.FromUser)
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

func (d *Demultiplexer) StartMonitoring(c *types.ClientInfo) {
	const ephemeralTimeout = 5 * time.Second

	// Monitor our channels - spawn workers if needed - more for messages obviously
	go monitorChannel(d.encenvelopeChannel, 20, 5, &d.envelopeWorkerCount, &d.mu,
		func() {
			ProcessEnvelopes(d.encenvelopeChannel, c, ephemeralTimeout, &d.envelopeWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.chatRequestChannel, 10, 3, &d.chatRequestWorkerCount, &d.mu,
		func() {
			ProcessChatRequests(d.chatRequestChannel, c, ephemeralTimeout, &d.chatRequestWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.chatConfirmChannel, 10, 3, &d.chatConfirmWorkerCount, &d.mu,
		func() {
			ProcessConfirmChatRequests(c, d.chatConfirmChannel, ephemeralTimeout, &d.chatConfirmWorkerCount, &d.mu)
		},
	)

	go monitorChannel(d.keyExchangeChannel, 10, 3, &d.keyExchangeRequestWorkerCount, &d.mu,
		func() {
			ProcessKeyExchangeRequests(c, d.keyExchangeChannel, ephemeralTimeout, &d.keyExchangeRequestWorkerCount, &d.mu) // TODO: This is a mes
		},
	)
}

func ProcessEnvelopes(ch <-chan *pb.EncryptedEnvelope, c *types.ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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

			msg, err := crypto.Decrypt(c, envelope.EncryptedMessage)
			if err != nil {
				log.Fatal("Failed to decrypt sealed message")
			}

			// fmt.Printf("[%s] [%s] [From:%s] : %s\n", envelope.SentAt.AsTime(), envelope.Chat.Name, envelope.FromUser, msg)
			// TODO: Batch insert messages?
			_, err = c.Pstatements.SaveMessage.ExecContext(context.TODO(), uuid.New().String(), envelope.Chat.Id, envelope.FromUser, c.UserID.String(), "inbound", msg, envelope.SentAt.AsTime().UnixMilli())
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

func ProcessChatRequests(ch <-chan *pb.BeginChatRequest, c *types.ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
			c.Cache.Invites[uuid.MustParse(chatRequest.InviteId)] = chatRequest
		case <-timeoutCh:
			fmt.Printf("ChatRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessConfirmChatRequests(c *types.ClientInfo, ch <-chan *pb.ConfirmChatRequest, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
				confirmerParsed := uuid.MustParse(confirmation.Confirmer)
				fmt.Printf("Invitation %v for:%s, With: %s, Status: Accepted\n", confirmation.InviteId, confirmation.Chat.Name, confirmerParsed)
				c.Cache.Chats[uuid.MustParse(confirmation.Chat.Id)] = confirmation.Chat
				c.Cache.Chats[uuid.MustParse(confirmation.Chat.Id)].State = pb.Chat_KEY_EXCHANGE_PENDING
				// TODO: Initiator will probably have to change
				InitiateKeyExchange(context.TODO(), c, confirmerParsed, confirmation.Chat)
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

func ProcessKeyExchangeRequests(c *types.ClientInfo, ch <-chan *pb.KeyExchangeRequest, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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

			fmt.Printf("keyExReq: %v", keyExReq)

			chatId := uuid.MustParse(keyExReq.ChatId)
			senderId := uuid.MustParse(keyExReq.SenderUserId)
			fmt.Printf("Key exchange initiated for: %v\n", chatId)

			chat, exists := c.Cache.Chats[chatId]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExReq.ChatId)
				return
			}

			participants := slices.Clone(chat.Participants)

			participantsSerialized := strings.Join(participants, ",")

			_, err := c.Pstatements.CreateChat.ExecContext(context.TODO(), chatId, chat.Name, uuid.MustParse(keyExReq.SenderUserId), participantsSerialized, chat.State.String())
			if err != nil {
				log.Fatal("Failed to save Chat")
			}

			// TODO: Signature is gross
			ReciprocateKeyExchange(context.TODO(), c, senderId, chat)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessKeyExchangeResponses(c *types.ClientInfo, ch <-chan *pb.KeyExchangeResponse, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
			chat, exists := c.Cache.Chats[uuid.MustParse(keyExRes.ChatId)]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExRes.ChatId)
				return
			}

			chat.State = pb.Chat_ENCRYPTED

			participants := slices.Clone(chat.Participants)

			participantsSerialized := strings.Join(participants, ",")

			_, err := c.Pstatements.CreateChat.ExecContext(context.TODO(), uuid.MustParse(keyExRes.ChatId), chat.Name, uuid.MustParse(keyExRes.ResponderUserId), participantsSerialized, chat.State.String())
			if err != nil {
				log.Fatal("Failed to save Chat")
			}

			// TODO: Something fails so the confirmations can be false???
			ConfirmKeyExchange(context.TODO(), c, uuid.MustParse(keyExRes.ResponderUserId), true, chat)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeResponse worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return
		}
	}
}

func ProcessKeyExchangeConfirmations(c *types.ClientInfo, ch <-chan *pb.KeyExchangeConfirmation, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) {
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
			chat, exists := c.Cache.Chats[uuid.MustParse(keyExCon.ChatId)]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExCon.ChatId)
				return
			}

			if chat.State != pb.Chat_ENCRYPTED {
				_, err := c.Pstatements.UpdateChatState.ExecContext(context.TODO(), pb.Chat_ENCRYPTED.String(), uuid.MustParse(keyExCon.ChatId))
				if err != nil {
					log.Fatal("Failed to save Chat")
				}

				chat.State = pb.Chat_ENCRYPTED

				ConfirmKeyExchange(context.TODO(), c, uuid.MustParse(keyExCon.ConfirmerUserId), true, chat)

			}
			if chat.State == pb.Chat_ENCRYPTED {
				fmt.Println("Chat already encrypted, confirmation skipped")
			}

			return

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
func monitorChannel[T any](ch <-chan T, threshold, maxWorkers int, workerCount *int, mu *sync.Mutex, spawnWorker func()) {
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

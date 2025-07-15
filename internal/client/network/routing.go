package network

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/JohnnyGlynn/strike/internal/client/crypto"
	"github.com/JohnnyGlynn/strike/internal/client/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

type Demultiplexer struct {
	friendRequestChannel           chan *pb.FriendRequest
	friendResponseChannel          chan *pb.FriendResponse
	encenvelopeChannel             chan *pb.EncryptedEnvelope
	keyExchangeChannel             chan *pb.KeyExchangeRequest
	keyExchangeResponseChannel     chan *pb.KeyExchangeResponse
	keyExchangeConfirmationChannel chan *pb.KeyExchangeConfirmation

	// Workers
	envelopeWorkerCount           int
	keyExchangeRequestWorkerCount int

	// mutex for workers
	// TODO: Global locking bad idea?
	mu sync.Mutex
}

func NewDemultiplexer(c *types.ClientInfo) *Demultiplexer {
	d := &Demultiplexer{
		friendRequestChannel:  make(chan *pb.FriendRequest, 200),
		friendResponseChannel: make(chan *pb.FriendResponse, 200),
		encenvelopeChannel:    make(chan *pb.EncryptedEnvelope, 200),
		// TODO: There has to be a better way
		keyExchangeChannel:             make(chan *pb.KeyExchangeRequest, 20),
		keyExchangeResponseChannel:     make(chan *pb.KeyExchangeResponse, 20),
		keyExchangeConfirmationChannel: make(chan *pb.KeyExchangeConfirmation, 20),
	}

	// Baseline workers
	d.mu.Lock()
	d.envelopeWorkerCount = 1
	d.keyExchangeRequestWorkerCount = 1
	d.mu.Unlock()

	// Run demultiplexer channel processors - Permanent processors
	go ProcessEnvelopes(d.encenvelopeChannel, c, 0, &d.envelopeWorkerCount, &d.mu)
	go ProcessFriendRequests(d.friendRequestChannel, c, 0, &d.keyExchangeRequestWorkerCount, &d.mu)
	go ProcessFriendResponse(d.friendResponseChannel, c, 0, &d.keyExchangeRequestWorkerCount, &d.mu)
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
	case *pb.StreamPayload_FriendRequest:
		select {
		case d.friendRequestChannel <- payload.FriendRequest:
		default:
			log.Printf("WARNING: Channel full - Friend Request dropped - Sender: %v\n", payload.FriendRequest.UserInfo.Username)
		}
	case *pb.StreamPayload_FriendResponse:
		select {
		case d.friendResponseChannel <- payload.FriendResponse:
		default:
			log.Printf("WARNING: Channel full - Friend Response dropped - Sender: %v\n", payload.FriendResponse.UserInfo.Username)
		}
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
		func() error {
			err := ProcessEnvelopes(d.encenvelopeChannel, c, ephemeralTimeout, &d.envelopeWorkerCount, &d.mu)
			if err != nil {
				return err
			}
			return nil
		},
	)

	go monitorChannel(d.keyExchangeChannel, 10, 3, &d.keyExchangeRequestWorkerCount, &d.mu,
		func() error {
			err := ProcessKeyExchangeRequests(c, d.keyExchangeChannel, ephemeralTimeout, &d.keyExchangeRequestWorkerCount, &d.mu)
			if err != nil {
				return err
			}
			return nil
		},
	)
}

func ProcessEnvelopes(ch <-chan *pb.EncryptedEnvelope, c *types.ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) error {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case envelope, ok := <-ch:
			if !ok {
				return nil
			}

			msg, err := crypto.Decrypt(c, envelope.EncryptedMessage)
			if err != nil {
				fmt.Printf("Failed to decrypt sealed message")
				return err
			}

			// TODO: Batch insert messages?
			if c.Shell.Mode == types.ModeChat && envelope.FromUser == c.Cache.CurrentChat.User.Id.String() {
				fmt.Printf("[%s]:%s\n", envelope.FromUser, msg)
			}

			_, err = c.Pstatements.SaveMessage.ExecContext(context.TODO(), uuid.New().String(), envelope.FromUser, c.UserID.String(), "inbound", msg, envelope.SentAt.AsTime().UnixMilli())
			if err != nil {
				fmt.Printf("Failed to save message")
				return err
			}
		case <-timeoutCh:
			fmt.Printf("Envelope worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return nil
		}
	}
}

func ProcessFriendRequests(ch <-chan *pb.FriendRequest, c *types.ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) error {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case friendRequest, ok := <-ch:
			if !ok {
				return nil
			}
			fmt.Printf("Friend Request from: %v\n", friendRequest.UserInfo.Username)
			// Recieve an invite, cache it
			c.Cache.FriendRequests[uuid.MustParse(friendRequest.InviteId)] = friendRequest
		case <-timeoutCh:
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return nil
		}
	}
}

func ProcessFriendResponse(ch <-chan *pb.FriendResponse, c *types.ClientInfo, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) error {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case friendRes, ok := <-ch:
			if !ok {
				return nil
			}
			fmt.Printf("Friend Response from: %v\n", friendRes.UserInfo.Username)

			delete(c.Cache.FriendRequests, uuid.MustParse(friendRes.InviteId))

			if friendRes.State {
				_, err := c.Pstatements.SaveUserDetails.ExecContext(context.TODO(), friendRes.UserInfo.UserId, friendRes.UserInfo.Username, friendRes.UserInfo.EncryptionPublicKey, friendRes.UserInfo.SigningPublicKey)
				if err != nil {
					fmt.Printf("failed adding to address book: %v", err)
					return err
				}

				InitiateKeyExchange(context.TODO(), c, uuid.MustParse(friendRes.UserInfo.UserId))
			}

		case <-timeoutCh:
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return nil
		}
	}
}

func ProcessKeyExchangeRequests(c *types.ClientInfo, ch <-chan *pb.KeyExchangeRequest, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) error {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case keyExReq, ok := <-ch:
			if !ok {
				return nil
			}

			fmt.Printf("keyExReq: %v", keyExReq)

			senderId := uuid.MustParse(keyExReq.SenderUserId)

			ReciprocateKeyExchange(context.TODO(), c, senderId)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeRequest worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return nil
		}
	}
}

func ProcessKeyExchangeResponses(c *types.ClientInfo, ch <-chan *pb.KeyExchangeResponse, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) error {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case keyExRes, ok := <-ch:
			if !ok {
				return nil
			}
			fmt.Printf("Key exchange response for: %v\n", keyExRes.ChatId)

			// TODO: Something fails so the confirmations can be false???
			ConfirmKeyExchange(context.TODO(), c, uuid.MustParse(keyExRes.ResponderUserId), true)

		case <-timeoutCh:
			fmt.Printf("KeyExchangeResponse worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return nil
		}
	}
}

func ProcessKeyExchangeConfirmations(c *types.ClientInfo, ch <-chan *pb.KeyExchangeConfirmation, idleTimeout time.Duration, workerCount *int, mu *sync.Mutex) error {
	for {
		var timeoutCh <-chan time.Time // channel for timer
		if idleTimeout > 0 {
			timeoutCh = time.After(idleTimeout) // if timeout non-0 create timout channel
		}
		select {
		case keyExCon, ok := <-ch:
			if !ok {
				return nil
			}
			chat, exists := c.Cache.Chats[uuid.MustParse(keyExCon.ChatId)]
			if !exists {
				log.Printf("Failed to find chat: %v", keyExCon.ChatId)
				return nil
			}

			if chat.State != pb.Chat_ENCRYPTED {
				_, err := c.Pstatements.UpdateChatState.ExecContext(context.TODO(), pb.Chat_ENCRYPTED.String(), uuid.MustParse(keyExCon.ChatId))
				if err != nil {
					fmt.Printf("Failed to save Chat")
					return err
				}

				chat.State = pb.Chat_ENCRYPTED

				ConfirmKeyExchange(context.TODO(), c, uuid.MustParse(keyExCon.ConfirmerUserId), true)

			}
			if chat.State == pb.Chat_ENCRYPTED {
				fmt.Println("Chat already encrypted, confirmation skipped")
			}

			return nil

		case <-timeoutCh:
			fmt.Printf("KeyExchangeResponse worker idle for %v, exiting.\n", idleTimeout) // shutdown ephemeral workers
			mu.Lock()
			*workerCount--
			mu.Unlock()
			return nil
		}
	}
}

// Generic Channel monitor- Provide it any channel and respective processor function
func monitorChannel[T any](ch <-chan T, threshold, maxWorkers int, workerCount *int, mu *sync.Mutex, spawnWorker func() error) {
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

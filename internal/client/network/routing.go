package network

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	// "time"

	"github.com/JohnnyGlynn/strike/internal/client/crypto"
	"github.com/JohnnyGlynn/strike/internal/client/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

type Demultiplexer struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	friendRequestChannel           chan *pb.FriendRequest
	friendResponseChannel          chan *pb.FriendResponse
	encenvelopeChannel             chan *pb.EncryptedEnvelope
	keyExchangeChannel             chan *pb.KeyExchangeRequest
	keyExchangeResponseChannel     chan *pb.KeyExchangeResponse
	keyExchangeConfirmationChannel chan *pb.KeyExchangeConfirmation

	workers map[string]int
	wrkMu   sync.Mutex
}

func NewDemultiplexer(c *types.ClientInfo) *Demultiplexer {

	ctx, cancel := context.WithCancel(context.Background())

	d := &Demultiplexer{
		ctx:                            ctx,
		cancel:                         cancel,
		workers:                        make(map[string]int),
		friendRequestChannel:           make(chan *pb.FriendRequest, 20),
		friendResponseChannel:          make(chan *pb.FriendResponse, 20),
		encenvelopeChannel:             make(chan *pb.EncryptedEnvelope, 200),
		keyExchangeChannel:             make(chan *pb.KeyExchangeRequest, 20),
		keyExchangeResponseChannel:     make(chan *pb.KeyExchangeResponse, 20),
		keyExchangeConfirmationChannel: make(chan *pb.KeyExchangeConfirmation, 20),
	}

	d.spawnWorker("encenv", func() { processEnvelopes(d.ctx, d.encenvelopeChannel, c) })
	d.spawnWorker("friendreq", func() { processFriendRequests(d.ctx, d.friendRequestChannel, c) })
	d.spawnWorker("friendres", func() { processFriendResponse(d.ctx, d.friendResponseChannel, c) })
	d.spawnWorker("keyEx", func() { processKeyExchangeRequests(d.ctx, d.keyExchangeChannel, c) })
	d.spawnWorker("keyExRes", func() { processKeyExchangeResponses(d.ctx, d.keyExchangeResponseChannel, c) })
	d.spawnWorker("keyExCon", func() { processKeyExchangeConfirmations(d.ctx, d.keyExchangeConfirmationChannel, c) })

	return d
}

func (d *Demultiplexer) spawnWorker(name string, fn func()) {
	d.wrkMu.Lock()
	defer d.wrkMu.Unlock()

	d.workers[name]++
	d.wg.Add(1)

	go func() {
		defer d.wg.Done()
		fn()
	}()
}

func (d *Demultiplexer) Shutdown() {
	d.cancel()
	d.wg.Wait()
	fmt.Println("Demux shutdown")
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

func processEnvelopes(ctx context.Context, ch <-chan *pb.EncryptedEnvelope, c *types.ClientInfo) error {
	for {
		select {
		case <-ctx.Done():
			return nil
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
				fmt.Printf("[%s]:%s\n", c.Cache.CurrentChat.User.Name, msg)
			}

			_, err = c.Pstatements.SaveMessage.ExecContext(ctx, uuid.New().String(), envelope.FromUser, "inbound", envelope.EncryptedMessage, envelope.SentAt.AsTime().UnixMilli())
			if err != nil {
				fmt.Printf("Failed to save message")
				return err
			}
		}
	}
}

func processFriendRequests(ctx context.Context, ch <-chan *pb.FriendRequest, c *types.ClientInfo) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case friendRequest, ok := <-ch:
			if !ok {
				return nil
			}
			fmt.Printf("Friend Request from: %v\n", friendRequest.UserInfo.Username)

			_, err := c.Pstatements.SaveFriendRequest.ExecContext(ctx, friendRequest.UserInfo.UserId, friendRequest.UserInfo.Username, friendRequest.UserInfo.EncryptionPublicKey, friendRequest.UserInfo.SigningPublicKey, "inbound")
			if err != nil {
				fmt.Printf("failed to save Friend Request")
				return err
			}
		}
	}
}

func processFriendResponse(ctx context.Context, ch <-chan *pb.FriendResponse, c *types.ClientInfo) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case friendRes, ok := <-ch:
			if !ok {
				return nil
			}
			fmt.Printf("Friend Response from: %v\n", friendRes.UserInfo.Username)

			if friendRes.State {
				_, err := c.Pstatements.SaveUserDetails.ExecContext(ctx, friendRes.UserInfo.UserId, friendRes.UserInfo.Username, friendRes.UserInfo.EncryptionPublicKey, friendRes.UserInfo.SigningPublicKey)
				if err != nil {
					fmt.Printf("failed adding to address book: %v", err)
					return err
				}

				InitiateKeyExchange(ctx, c, uuid.MustParse(friendRes.UserInfo.UserId))
			}

			_, err := c.Pstatements.DeleteFriendRequest.ExecContext(ctx, friendRes.UserInfo.UserId)
			if err != nil {
				fmt.Printf("failed deleting friend request: %v", err)
				return err
			}

		}
	}
}

func processKeyExchangeRequests(ctx context.Context, ch <-chan *pb.KeyExchangeRequest, c *types.ClientInfo) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case keyExReq, ok := <-ch:
			if !ok {
				return nil
			}

			fmt.Printf("keyExReq: %v", keyExReq)

			senderId := uuid.MustParse(keyExReq.SenderUserId)

			ReciprocateKeyExchange(ctx, c, senderId)
		}
	}
}

func processKeyExchangeResponses(ctx context.Context, ch <-chan *pb.KeyExchangeResponse, c *types.ClientInfo) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case keyExRes, ok := <-ch:
			if !ok {
				return nil
			}

			// TODO: Something fails so the confirmations can be false???
			err := ConfirmKeyExchange(ctx, c, uuid.MustParse(keyExRes.ResponderUserId), true)
			if err != nil {
				fmt.Println("key exchange confirmation failed")
				return err
			}

		}
	}
}

func processKeyExchangeConfirmations(ctx context.Context, ch <-chan *pb.KeyExchangeConfirmation, c *types.ClientInfo) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case keyExCon, ok := <-ch:
			if !ok {
				return nil
			}

			var confirmed int
			err := c.Pstatements.GetKeyEx.QueryRowContext(ctx, keyExCon.ConfirmerUserId).Scan(&confirmed)
			if err != nil && err != sql.ErrNoRows {
				fmt.Printf("failed to query key exchange state")
				return err
			}

			if confirmed != 0 {
				fmt.Println("Keys have already been exchanged")
				return nil
			}

			_, err = c.Pstatements.ConfirmKeyEx.ExecContext(ctx, true, keyExCon.ConfirmerUserId)
			if err != nil {
				fmt.Println("failed to confirm key exchange locally")
				return err
				//TODO: Retry mechanism?
			}

			err = ConfirmKeyExchange(ctx, c, uuid.MustParse(keyExCon.ConfirmerUserId), true)
			if err != nil {
				fmt.Println("key exchange confirmation failed")
				return err
			}

			//Username
			fmt.Printf("Keys have been exchanged with %s\n", keyExCon.ConfirmerUserId)

			return nil
		}
	}
}

// // Generic Channel monitor- Provide it any channel and respective processor function
// func monitorChannel[T any](ch <-chan T, threshold, maxWorkers int, workerCount *int, mu *sync.Mutex, spawnWorker func() error) {
// 	ticker := time.NewTicker(5 * time.Second) // Check channel every 5 seconds
// 	defer ticker.Stop()
// 	for range ticker.C {
// 		mu.Lock()
// 		if len(ch) > threshold && *workerCount < maxWorkers {
// 			*workerCount++
// 			log.Printf("Spawning new ephemeral worker; current workers: %d", *workerCount)
// 			// Callback generic for any of the Process* functions
// 			go spawnWorker()
// 		}
// 		mu.Unlock()
// 	}
// }

// func (d *Demultiplexer) StartMonitoring(c *types.ClientInfo) {
// 	const ephemeralTimeout = 5 * time.Second

// 	// Monitor our channels - spawn workers if needed - more for messages obviously
// 	go monitorChannel(d.encenvelopeChannel, 20, 5, &d.envelopeWorkerCount, &d.mu,
// 		func() error {
// 			err := ProcessEnvelopes(d.encenvelopeChannel, c, ephemeralTimeout, &d.envelopeWorkerCount, &d.mu)
// 			if err != nil {
// 				return err
// 			}
// 			return nil
// 		},
// 	)

// 	go monitorChannel(d.keyExchangeChannel, 10, 3, &d.keyExchangeRequestWorkerCount, &d.mu,
// 		func() error {
// 			err := ProcessKeyExchangeRequests(c, d.keyExchangeChannel, ephemeralTimeout, &d.keyExchangeRequestWorkerCount, &d.mu)
// 			if err != nil {
// 				return err
// 			}
// 			return nil
// 		},
// 	)
// }

package network

import (
	"context"
	"database/sql"
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

type routeBinding[T any] struct {
	name        string
	channel     <-chan T
	threshold   int
	maxWorkers  int
	processor   func(msg T)
	handler     func(ctx context.Context, ch <-chan T, c *types.Client)
	idleTimeout time.Duration
}

func NewDemultiplexer(c *types.Client) *Demultiplexer {

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

	mux := demuxRoutes(d, c)

	//TODO: Make registerRoute generic?
	for _, r := range mux {
		switch rtype := r.(type) {
		case routeBinding[*pb.EncryptedEnvelope]:
			registerRoute(d, rtype, c)
		case routeBinding[*pb.FriendRequest]:
			registerRoute(d, rtype, c)
		case routeBinding[*pb.FriendResponse]:
			registerRoute(d, rtype, c)
		case routeBinding[*pb.KeyExchangeRequest]:
			registerRoute(d, rtype, c)
		case routeBinding[*pb.KeyExchangeResponse]:
			registerRoute(d, rtype, c)
		case routeBinding[*pb.KeyExchangeConfirmation]:
			registerRoute(d, rtype, c)
		default:
			fmt.Printf("route not found %T", r)
		}

		return d
	}

	return nil
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

func spwanEphemeral[T any](d *Demultiplexer, name string, ch <-chan T, processor func(msg T), idleTimeout time.Duration) {
	d.wrkMu.Lock()
	d.workers[name]++
	d.wg.Add(1)
	d.wrkMu.Unlock()

	go func() {
		defer func() {
			d.wrkMu.Lock()
			d.workers[name]--
			d.wrkMu.Unlock()
			d.wg.Done()
			log.Printf("ephemeral %s shutdown...", name)
		}()

		timer := time.NewTimer(idleTimeout)
		defer timer.Stop()

		for {
			select {
			case <-d.ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(idleTimeout)
				processor(msg)
			case <-timer.C:
				fmt.Println("time out")
				return
			}
		}
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

func processEnvelope(ctx context.Context, env *pb.EncryptedEnvelope, c *types.Client) error {
	msg, err := crypto.Decrypt(c, env.EncryptedMessage)
	if err != nil {
		fmt.Printf("Failed to decrypt sealed message")
		return err
	}

	// TODO: Batch insert messages?
	if c.State.Shell.Mode == types.ModeChat && env.FromUser == c.State.Cache.CurrentChat.User.Id.String() {
		fmt.Printf("[%s]:%s\n", c.State.Cache.CurrentChat.User.Name, msg)
	}

	_, err = c.DB.Messages.SaveMessage.ExecContext(ctx, uuid.New().String(), env.FromUser, "inbound", env.EncryptedMessage, env.SentAt.AsTime().UnixMilli())
	if err != nil {
		fmt.Printf("Failed to save message")
		return err
	}

	return nil
}

func processFriendRequest(ctx context.Context, fr *pb.FriendRequest, c *types.Client) error {

	fmt.Printf("Friend Request from: %v\n", fr.UserInfo.Username)

	_, err := c.DB.FriendRequest.SaveFriendRequest.ExecContext(ctx, fr.UserInfo.UserId, fr.UserInfo.Username, fr.UserInfo.EncryptionPublicKey, fr.UserInfo.SigningPublicKey, "inbound")
	if err != nil {
		fmt.Printf("failed to save Friend Request")
		return err
	}

	return nil

}

func processFriendResponse(ctx context.Context, fr *pb.FriendResponse, c *types.Client) error {

	fmt.Printf("Friend Response from: %v\n", fr.UserInfo.Username)

	if fr.State {
		_, err := c.DB.Friends.SaveUserDetails.ExecContext(ctx, fr.UserInfo.UserId, fr.UserInfo.Username, fr.UserInfo.EncryptionPublicKey, fr.UserInfo.SigningPublicKey)
		if err != nil {
			fmt.Printf("failed adding to address book: %v", err)
			return err
		}

    err = InitiateKeyExchange(ctx, c, uuid.MustParse(fr.UserInfo.UserId))
    if err != nil {
      return err
    }

	}

	_, err := c.DB.FriendRequest.DeleteFriendRequest.ExecContext(ctx, fr.UserInfo.UserId)
	if err != nil {
		fmt.Printf("failed deleting friend request: %v", err)
		return err
	}

	return nil
}

func processKeyExchangeRequest(ctx context.Context, kx *pb.KeyExchangeRequest, c *types.Client) error {

	u := types.User{}

	var created time.Time
	row := c.DB.Friends.GetUser.QueryRowContext(context.TODO(), kx.SenderUserId)
  err := row.Scan(&u.Id, &u.Name, &u.Enckey, &u.Sigkey, &u.KeyEx, &created)
	if err != nil {
		return fmt.Errorf("an error occured: %v", err)
	}

  if !crypto.VerifyEdSignatures(u.Sigkey, kx.Nonce, kx.CurvePublicKey, kx.Signatures){
    return fmt.Errorf("failed to verify signatures")
  }

	senderId := uuid.MustParse(kx.SenderUserId)

  err = ReciprocateKeyExchange(ctx, c, senderId)
  if err != nil {
    return err
  }

	return nil
}

func processKeyExchangeResponse(ctx context.Context, kx *pb.KeyExchangeResponse, c *types.Client) error {

	u := types.User{}

	var created time.Time
	row := c.DB.Friends.GetUser.QueryRowContext(context.TODO(), kx.ResponderUserId)
  err := row.Scan(&u.Id, &u.Name, &u.Enckey, &u.Sigkey, &u.KeyEx, &created)
	if err != nil {
		return fmt.Errorf("an error occured: %v", err)
	}

  if !crypto.VerifyEdSignatures(u.Sigkey, kx.Nonce, kx.CurvePublicKey, kx.Signatures){
    return fmt.Errorf("failed to verify signatures")
  }

	err = ConfirmKeyExchange(ctx, c, uuid.MustParse(kx.ResponderUserId), true)
	if err != nil {
		fmt.Println("key exchange confirmation failed")
		return err
	}

	return nil

}

func processKeyExchangeConfirmation(ctx context.Context, kx *pb.KeyExchangeConfirmation, c *types.Client) error {
	var confirmed int
	err := c.DB.Friends.GetKeyEx.QueryRowContext(ctx, kx.ConfirmerUserId).Scan(&confirmed)
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("failed to query key exchange state")
		return err
	}

	if confirmed != 0 {
		fmt.Println("Keys have already been exchanged")
		return nil
	}

	_, err = c.DB.Friends.ConfirmKeyEx.ExecContext(ctx, true, kx.ConfirmerUserId)
	if err != nil {
		fmt.Println("failed to confirm key exchange locally")
		return err
		//TODO: Retry mechanism?
	}

	err = ConfirmKeyExchange(ctx, c, uuid.MustParse(kx.ConfirmerUserId), true)
	if err != nil {
		fmt.Println("key exchange confirmation failed")
		return err
	}

	//Username
	fmt.Printf("Keys have been exchanged with %s\n", kx.ConfirmerUserId)

	return nil
}

func registerRoute[T any](d *Demultiplexer, binding routeBinding[T], c *types.Client) {
	d.spawnWorker(binding.name, func() {
		binding.handler(d.ctx, binding.channel, c) //runs "forever"
	})

	autoScaler(
		d,
		binding.name,
		binding.channel,
		binding.threshold,
		binding.maxWorkers,
		binding.processor,
		binding.idleTimeout,
	)
}

func demuxRoutes(d *Demultiplexer, c *types.Client) []any {
	return []any{
		routeBinding[*pb.EncryptedEnvelope]{
			name:        "encenv",
			channel:     d.encenvelopeChannel,
			threshold:   20,
			maxWorkers:  5,
			idleTimeout: 10 * time.Second,
			processor: func(msg *pb.EncryptedEnvelope) {
        err := processEnvelope(d.ctx, msg, c)
        if err != nil {
          return
        }
			},
			handler: func(ctx context.Context, ch <-chan *pb.EncryptedEnvelope, c *types.Client) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg := <-ch:
            err := processEnvelope(ctx, msg, c)
            if err != nil {
              return
            }
					}
				}
			},
		},
		routeBinding[*pb.FriendRequest]{
			name:        "friendreq",
			channel:     d.friendRequestChannel,
			threshold:   5,
			maxWorkers:  2,
			idleTimeout: 1 * time.Second,
			processor: func(msg *pb.FriendRequest) {
        err := processFriendRequest(d.ctx, msg, c)
        if err != nil {
          return
        }
			},
			handler: func(ctx context.Context, ch <-chan *pb.FriendRequest, c *types.Client) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg := <-ch:
            err := processFriendRequest(ctx, msg, c)
            if err != nil {
              return
            }
					}
				}
			},
		},
		routeBinding[*pb.FriendResponse]{
			name:        "friendres",
			channel:     d.friendResponseChannel,
			threshold:   5,
			maxWorkers:  2,
			idleTimeout: 1 * time.Second,
			processor: func(msg *pb.FriendResponse) {
        err := processFriendResponse(d.ctx, msg, c)
        if err != nil {
          return
        }
			},
			handler: func(ctx context.Context, ch <-chan *pb.FriendResponse, c *types.Client) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg := <-ch:
            err := processFriendResponse(ctx, msg, c)
            if err != nil {
              return
            }
					}
				}
			},
		},
		routeBinding[*pb.KeyExchangeRequest]{
			name:        "kxreq",
			channel:     d.keyExchangeChannel,
			threshold:   5,
			maxWorkers:  2,
			idleTimeout: 1 * time.Second,
			processor: func(msg *pb.KeyExchangeRequest) {
        err :=  processKeyExchangeRequest(d.ctx, msg, c)
        if err != nil {
          return
        }
			},
			handler: func(ctx context.Context, ch <-chan *pb.KeyExchangeRequest, c *types.Client) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg := <-ch:
            err := processKeyExchangeRequest(ctx, msg, c)
            if err != nil {
              return
            }
					}
				}
			},
		},
		routeBinding[*pb.KeyExchangeResponse]{
			name:        "kxres",
			channel:     d.keyExchangeResponseChannel,
			threshold:   5,
			maxWorkers:  2,
			idleTimeout: 1 * time.Second,
			processor: func(msg *pb.KeyExchangeResponse) {
				err := processKeyExchangeResponse(d.ctx, msg, c)
        if err != nil {
          return
        }
			},
			handler: func(ctx context.Context, ch <-chan *pb.KeyExchangeResponse, c *types.Client) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg := <-ch:
            err := processKeyExchangeResponse(ctx, msg, c)
            if err != nil {
              return
            }
					}
				}
			},
		},
		routeBinding[*pb.KeyExchangeConfirmation]{
			name:        "kxcon",
			channel:     d.keyExchangeConfirmationChannel,
			threshold:   5,
			maxWorkers:  2,
			idleTimeout: 1 * time.Second,
			processor: func(msg *pb.KeyExchangeConfirmation) {
				err := processKeyExchangeConfirmation(d.ctx, msg, c)
        if err != nil {
          return
        }
			},
			handler: func(ctx context.Context, ch <-chan *pb.KeyExchangeConfirmation, c *types.Client) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg := <-ch:
            err := processKeyExchangeConfirmation(ctx, msg, c)
            if err != nil {
              return
            }
					}
				}
			},
		},
		//Expansion
		// routeBinding[*pb.]{
		// 	name:        "",
		// 	channel:     d.Channel,
		// 	threshold:   5,
		// 	maxWorkers:  2,
		// 	idleTimeout: 1 * time.Second,
		// 	processor: func(msg *pb.) {
		// 		process(d.ctx, msg, c)
		// 	},
		// 	handler: func(ctx context.Context, ch <-chan *pb., c *types.Client) {
		// 		for {
		// 			select {
		// 			case <-ctx.Done():
		// 				return
		// 			case msg := <-ch:
		// 				process(ctx, msg, c)
		// 			}
		// 		}
		// 	},
		// },
	}
}

func autoScaler[T any](d *Demultiplexer, name string, ch <-chan T, threshold int, maxWorkers int, processor func(msg T), idleTimeout time.Duration) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-d.ctx.Done():
				log.Printf("%s payload monitor shutting down", name)
				return
			case <-ticker.C:
				d.wrkMu.Lock()
				workerCount := d.workers[name]
				if len(ch) > threshold && workerCount < maxWorkers {
					log.Printf("Spawning ephemeral worker %d (channel: %s)", len(ch), name)
					spwanEphemeral(d, name, ch, processor, idleTimeout)
				}
				d.wrkMu.Unlock()
			}
		}

	}()
}


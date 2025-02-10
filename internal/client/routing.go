package client

import (
	"fmt"
	"log"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type Demultiplexer struct {
	chatRequestChannel chan *pb.BeginChatRequest
	chatConfirmChannel chan *pb.ConfirmChatRequest
	envelopeChannel    chan *pb.Envelope
}

func NewDemultiplexer() *Demultiplexer {
	return &Demultiplexer{
		chatRequestChannel: make(chan *pb.BeginChatRequest, 20),
		chatConfirmChannel: make(chan *pb.ConfirmChatRequest, 20),
		envelopeChannel:    make(chan *pb.Envelope, 200),
	}
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
	default:
		log.Println("Unknown payload type")
	}
}

func ProcessEnvelopes(ch <-chan *pb.Envelope) {
	for envelope := range ch {
		fmt.Printf("[%s] [%s] [From:%s] : %s\n", envelope.SentAt.AsTime(), envelope.Chat.Name, envelope.FromUser, envelope.Message)
	}
}

func ProcessChatRequests(ch <-chan *pb.BeginChatRequest, cache map[string]*pb.BeginChatRequest) {
	for chatRequest := range ch {
		fmt.Printf("Chat Invite recieved from:%v Chat Name: %v\n", chatRequest.Initiator, chatRequest.Chat.Name)
		// Recieve an invite, cache it
		cache[chatRequest.InviteId] = chatRequest
	}
}

func ProcessConfirmChatRequests(ch <-chan *pb.ConfirmChatRequest, cache map[string]*pb.Chat) {
	for confirmation := range ch {
		if confirmation.State {
			fmt.Printf("Invitation %v for:%s, With: %s, Status: Accepted\n", confirmation.InviteId, confirmation.Chat.Name, confirmation.Confirmer)
			cache[confirmation.Chat.Id] = confirmation.Chat
		} else {
			fmt.Printf("Invitation %v for:%s, With: %s, Status: Declined\n", confirmation.InviteId, confirmation.Chat.Name, confirmation.Confirmer)
		}
	}
}

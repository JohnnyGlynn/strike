package client

import (
	"fmt"
	"log"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type Demultiplexer struct {
	chatRequestChannel chan *pb.BeginChatRequest
	chatConfirmChannel chan *pb.ConfirmChatRequest
	envelopeChannel   chan *pb.Envelope
}

func NewDemultiplexer() *Demultiplexer {
	return &Demultiplexer{
		chatRequestChannel: make(chan *pb.BeginChatRequest, 20),
		chatConfirmChannel: make(chan *pb.ConfirmChatRequest, 20),
		envelopeChannel:   make(chan *pb.Envelope, 200),
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
      //TODO: Retry to create chats if this fails
    }    
  }
}

func ProcessEnvelopes(ch <-chan *pb.Envelope) {
  for envelopes := range ch {
    //TODO:Do Stuff 
    fmt.Printf("TODO: %s", envelopes.Message)
  }
}

func ProcessChatRequests(ch <-chan *pb.BeginChatRequest) {
  for chatRequests := range ch {
    //TODO:Do Stuff 
    fmt.Printf("TODO: %s", chatRequests.Chat)
  }
}

func ProcessConfirmChatRequests(ch <-chan *pb.ConfirmChatRequest) {
  for confirmation := range ch {
    //TODO:Do Stuff 
    fmt.Printf("TODO: %s", confirmation.Chat)
  }
}


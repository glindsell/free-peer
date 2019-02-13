package lib

import (
	"fmt"
	"log"
	"sync"
	pb "github.com/chainforce/free-peer/single-msg-type/chaincode_proto"
	"errors"
)

type ConnectionHandler struct {
	sync.Mutex
	Connection *Connection
	chatClient *pb.Chaincode_ChaincodeChatClient // allows gRPC stream
	OngoingTxs map[int32]chan *pb.ChaincodeMessage
}

func (h *ConnectionHandler) catchResponse(tx *pb.ChaincodeMessage) {
	for {
		resp, err := h.recvResp()
		if err != nil {
			log.Fatalf("%v", err)
		}
		if resp.TxID != tx.TxID {
			log.Fatalf("error: bad req, txid mismatch")
		}
		if resp.Message == "CHAINCODE DONE" {
			err = h.closeSend()
			if err != nil {
				log.Fatalf("%v", err)
			}
			h.Connection.Lock()
			h.Connection.Requests--
			h.Connection.Unlock()
			return
		}
		reqMessage := fmt.Sprintf("PEER RESPONSE OK to: %v", resp.Message)
		req := &pb.ChaincodeMessage{Message: reqMessage, IsTX: false, TxID: resp.TxID}

		err = h.sendReq(req)
		if err != nil {
			log.Fatalf("%v", err)
		}
	}
}

func (h *ConnectionHandler) SendTx(message *pb.ChaincodeMessage) error {
	h.Connection.Lock()
	h.Connection.Requests++
	h.Connection.Unlock()
	go h.catchResponse(message)
	if err := h.sendReq(message); err != nil {
		return err
	}
	return nil
}

func (h *ConnectionHandler) sendReq(request *pb.ChaincodeMessage) error {
	h.Lock()
	defer h.Unlock()
	log.Printf(" | SEND >>> %v on connection: %v", request.Message, h.Connection.Id)
	chatClient := *h.chatClient
	if err := chatClient.Send(request); err != nil {
		return errors.New(fmt.Sprintf("failed to send request on connection %v: %v", h.Connection.Id, err))
	}
	return nil
}

func (h *ConnectionHandler) recvResp() (*pb.ChaincodeMessage, error) {
	h.Lock()
	defer h.Unlock()
	chatClient := *h.chatClient
	in, err := chatClient.Recv()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to receive response on connection %v: %v", h.Connection.Id, err))
	}
	log.Printf(" | REVC <<< %v on connection: %v", in.Message, h.Connection.Id)
	return in, nil
}

func (h *ConnectionHandler) closeSend() error {
	h.Lock()
	defer h.Unlock()
	chatClient := *h.chatClient
	err := chatClient.CloseSend()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to send close on stream: %v", err))
	}
	return nil
}

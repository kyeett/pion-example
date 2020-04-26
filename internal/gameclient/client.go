package gameclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/kyeett/pion-example/internal/signalingserver"
	"github.com/pion/webrtc/v2"
)

type Client struct {
	clientID       string
	wsConn         *websocket.Conn
	webrtcConfig   webrtc.Configuration
	peers          *PeerMap
	messageHandler func(msg []byte) error
}

func New(u url.URL, roomID string, messageHandler func(msg []byte) error) (*Client, error) {
	u.Path = "/room/" + roomID
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}

	var init signalingserver.InitMessage
	if err = c.ReadJSON(&init); err != nil {
		log.Fatal("failed decoding:", err)
	}

	if init.ClientID == "" {
		log.Fatal("invalid clientID:", err)
	}

	// Setup WebRTC
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	return &Client{
		clientID:       init.ClientID,
		wsConn:         c,
		webrtcConfig:   config,
		peers:          NewPeerMap(),
		messageHandler: messageHandler,
	}, nil
}

func (c *Client) SendMessage(msg []byte) error {
	c.peers.Broadcast(msg)
	return nil
}

type TypeMessage struct {
	Type string `json:"type"`
}

func (c *Client) Start() {
	for {
		_, message, err := c.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var typ TypeMessage
		if err := json.Unmarshal(message, &typ); err != nil {
			log.Printf("unmarshalling: %v", err)
			continue
		}

		log.Printf("Received a message of type: %q\n", typ.Type)
		switch typ.Type {
		case "offer":
			if err := c.handleOffer(message); err != nil {
				log.Printf("failed handling offer: %v", err)
				continue
			}

		case "answer":
			if err := c.handleAnswer(message); err != nil {
				log.Printf("failed handling answer: %v", err)
				continue
			}

		case "new_client":
			if err := c.handleNewClient(message); err != nil {
				log.Printf("failed handling new client: %v", err)
				continue
			}
		default:
			log.Printf("unhandeled message type: %v", err)
			continue
		}
	}
}

func (c *Client) handleNewClient(message []byte) error {
	var msg signalingserver.NewClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}

	if msg.ClientID == "" {
		return errors.New("invalid client ID")
	}

	return c.setupNewConnectionAndSendOffer(msg.ClientID)
}

func (c *Client) handleAnswer(message []byte) error {
	var msg signalingserver.SessionDescriptionMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}

	return c.peers.UpdateDescription(msg.Source, msg.SessionDescription)
}

func (c *Client) handleOffer(message []byte) error {
	var msg signalingserver.SessionDescriptionMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}

	return c.setupNewConnectionAndSendAnswer(msg.Source, msg.SessionDescription)
}

func (c *Client) setupNewConnectionAndSendOffer(clientID string) error {
	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(c.webrtcConfig)
	if err != nil {
		return err
	}

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		return err
	}

	// TODO: Clean up
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())
		_ = c.peers.UpdateDataChannel(clientID, dataChannel)
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		c.messageHandler(msg.Data)
	})

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		return err
	}

	// Save peer
	if err := c.peers.New(clientID, peerConnection, nil); err != nil {
		return err
	}

	offerMessage := signalingserver.SessionDescriptionMessage{
		SessionDescription: offer,
		Target:             clientID,
		Source:             c.clientID,
	}

	return c.wsConn.WriteJSON(offerMessage)
}

func (c *Client) setupNewConnectionAndSendAnswer(clientID string, offer webrtc.SessionDescription) error {
	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(c.webrtcConfig)
	if err != nil {
		return err
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open.\n", d.Label(), d.ID())
			_ = c.peers.UpdateDataChannel(clientID, d)
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			_ = c.messageHandler(msg.Data)
		})
	})

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Save peer
	if err := c.peers.New(clientID, peerConnection, nil); err != nil {
		return err
	}

	answerMessage := signalingserver.SessionDescriptionMessage{
		SessionDescription: answer,
		Target:             clientID,
		Source:             c.clientID,
	}

	return c.wsConn.WriteJSON(answerMessage)
}

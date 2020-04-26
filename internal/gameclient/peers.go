package gameclient

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v2"
)

type Peer struct {
	id string
	*webrtc.DataChannel
	*webrtc.PeerConnection
}

type PeerMap struct {
	conns map[string]*Peer
	lock  *sync.Mutex
}

func NewPeerMap() *PeerMap {
	return &PeerMap{
		conns: map[string]*Peer{},
		lock:  &sync.Mutex{},
	}
}

func (m *PeerMap) New(id string, connection *webrtc.PeerConnection, channel *webrtc.DataChannel) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, exists := m.conns[id]
	if exists {
		return errors.New("peer with ID already exists")
	}

	p := Peer{
		id:             id,
		DataChannel:    channel,
		PeerConnection: connection,
	}

	m.conns[id] = &p
	return nil
}

func (m *PeerMap) UpdateDescription(id string, desc webrtc.SessionDescription) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	peer, exists := m.conns[id]
	if !exists {
		return errors.New("peer with ID doesn't exist")
	}

	return peer.PeerConnection.SetRemoteDescription(desc)
}

func (m *PeerMap) UpdateDataChannel(id string, channel *webrtc.DataChannel) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	peer, exists := m.conns[id]
	if !exists {
		return errors.New("peer with ID doesn't exist")
	}

	peer.DataChannel = channel
	return nil
}

func (m *PeerMap) Broadcast(msg []byte) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, peer := range m.conns {
		if peer.DataChannel == nil {
			continue
		}

		if err := peer.DataChannel.Send(msg); err != nil {
			fmt.Println("failed to send message", err)
		}
	}
}

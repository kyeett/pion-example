package signalingserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/webrtc/v2"
	"net/http"
	"sync"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
	"gopkg.in/olahol/melody.v1"
)

type User struct {
	UID uuid.UUID `json:"uid"`
}

type Server struct {
	router   http.Handler
	m        *melody.Melody
	lock     *sync.Mutex
	sessions map[string]*melody.Session
}

const (
	keyRoomID   = "room_id"
	keyClientID = "client_id"
)

func New() *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)

	m := melody.New()
	m.Config.MaxMessageSize = 2048
	r.Get("/room/{room_id}", func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "room_id")
		m.HandleRequestWithKeys(w, r, map[string]interface{}{
			keyRoomID: roomID,
		})
	})

	s := &Server{
		router:   r,
		m:        m,
		lock:     new(sync.Mutex),
		sessions: map[string]*melody.Session{},
	}

	m.HandleConnect(s.handleConnect)
	m.HandleMessage(s.handleMessage)

	return s
}

var (
	offerType  = "offer"
	answerType = "answer"
)

func (s *Server) handleMessage(sess *melody.Session, msg []byte) {
	var mp map[string]string
	if err := json.Unmarshal(msg, &mp); err != nil {
		fmt.Printf("Error\n", err)
		return
	}

	switch mp["type"] {
	case offerType:
		target := mp["target"]
		if err := s.sendOffer(target, msg); err != nil {
			fmt.Printf("Error when sending offer: %s\n", err)
			return
		}
	case answerType:
		target := mp["target"]
		if err := s.sendAnswer(target, msg); err != nil {
			fmt.Printf("Error when sending answer: %s\n", err)
			return
		}

	default:
		fmt.Printf("Invalid message type: %q\n", mp["type"])
		//s.m.BroadcastOthers([]byte(msg), sess)
	}
}

type Offer struct {
}

func (s *Server) sendOffer(clientID string, msg []byte) error {
	sess, err := s.getSession(clientID)
	if err != nil {
		return errors.New("session not found")
	}

	if err := sess.Write(msg); err != nil {
		return err
	}

	return nil
}

func (s *Server) sendAnswer(clientID string, msg []byte) error {
	sess, err := s.getSession(clientID)
	if err != nil {
		return errors.New("session not found")
	}

	if err := sess.Write(msg); err != nil {
		return err
	}

	return nil
}

func (s *Server) getSession(clientID string) (*melody.Session, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	target, found := s.sessions[clientID]
	if !found {
		return nil, errors.New("session not found")
	}
	return target, nil
}

func isSameRoom(self, other *melody.Session) bool {
	selfRoom, _ := self.Get(keyRoomID)
	otherRoom, _ := other.Get(keyRoomID)
	return selfRoom == otherRoom
}

type SessionDescriptionMessage struct {
	webrtc.SessionDescription
	Target string `json:"target"`
	Source string `json:"source"`
}

type InitMessage struct {
	Type     string `json:"type"`
	ClientID string `json:"client_id"`
}

type NewClientMessage InitMessage

func (s *Server) handleConnect(sess *melody.Session) {
	newClientID := uuid.New().String()
	s.lock.Lock()
	s.sessions[newClientID] = sess
	sess.Set(keyClientID, newClientID)
	s.lock.Unlock()

	initMessage := InitMessage{
		Type:     "init",
		ClientID: newClientID,
	}

	b, err := json.Marshal(initMessage)
	if err != nil {
		fmt.Printf("Error when marshalling init message: %s\n", err)
		return
	}

	if err := sess.Write(b); err != nil {
		fmt.Printf("Error init connection message: %s\n", err)
		sess.CloseWithMsg([]byte("something went wrong"))
		return
	}
	fmt.Printf("Created new client with id %q, %q\n", newClientID, b)

	newClientMessage := NewClientMessage{
		Type:     "new_client",
		ClientID: newClientID,
	}
	b, err = json.Marshal(newClientMessage)
	if err != nil {
		fmt.Printf("Error when marshalling init message: %s\n", err)
		return
	}
	filterToRoom := func(other *melody.Session) bool {
		return other != sess && isSameRoom(sess, other)
	}
	if err = s.m.BroadcastFilter(b, filterToRoom); err != nil {
		fmt.Printf("Error broadcasting connection message: %s\n", err)
		return
	}
}

func (s *Server) Start() error {
	return http.ListenAndServe(":5000", s.router)
}

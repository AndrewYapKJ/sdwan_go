package shared

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Peer struct {
	ID        string    `json:"id"`
	PublicKey string    `json:"publicKey"`
	Address   string    `json:"address"`
	Endpoint  string    `json:"endpoint,omitempty"`
	LastSeen  time.Time `json:"lastSeen"`
}

type HubState struct {
	HubPubKey   string
	HubPrivKey  wgtypes.Key
	Peers       map[string]*Peer
	Tokens      map[string]time.Time
	Subnet      *net.IPNet
	NextHostOct uint8
	Mu          sync.RWMutex
}

func NewHubState() (*HubState, error) {
	_, ipnet, err := net.ParseCIDR("10.42.0.0/24")
	if err != nil {
		return nil, err
	}
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &HubState{
		HubPrivKey:  priv,
		HubPubKey:   priv.PublicKey().String(),
		Peers:       make(map[string]*Peer),
		Tokens:      make(map[string]time.Time),
		Subnet:      ipnet,
		NextHostOct: 2, // .1 reserved for hub
	}, nil
}

func (h *HubState) GenerateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	h.Mu.Lock()
	h.Tokens[t] = time.Now().Add(10 * time.Minute)
	h.Mu.Unlock()
	return t
}

func (h *HubState) ValidateToken(t string) bool {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	exp, ok := h.Tokens[t]
	if !ok || time.Now().After(exp) {
		return false
	}
	delete(h.Tokens, t) // one-time use
	return true
}

func (h *HubState) AllocatePeer(pub string) (*Peer, error) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	if _, exists := h.Peers[pub]; exists {
		p := h.Peers[pub]
		p.LastSeen = time.Now()
		return p, nil
	}
	if h.NextHostOct == 255 {
		return nil, errors.New("address pool exhausted")
	}
	ip := make(net.IP, len(h.Subnet.IP))
	copy(ip, h.Subnet.IP)
	ip[3] = h.NextHostOct
	h.NextHostOct++
	p := &Peer{
		ID:        pub[:8],
		PublicKey: pub,
		Address:   ip.String() + "/24",
		LastSeen:  time.Now(),
	}
	h.Peers[pub] = p
	return p, nil
}

func (h *HubState) SetPeerEndpoint(pub, endpoint string) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	if p, ok := h.Peers[pub]; ok {
		p.Endpoint = endpoint
		p.LastSeen = time.Now()
	}
}

func (h *HubState) Touch(pub string) {
	h.Mu.Lock()
	if p, ok := h.Peers[pub]; ok {
		p.LastSeen = time.Now()
	}
	h.Mu.Unlock()
}

func (h *HubState) ListPeers() []*Peer {
	h.Mu.RLock()
	out := make([]*Peer, 0, len(h.Peers))
	for _, p := range h.Peers {
		out = append(out, p)
	}
	h.Mu.RUnlock()
	return out
}

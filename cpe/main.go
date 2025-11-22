package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type registerResp struct {
	AssignedAddress string `json:"assignedAddress"`
	HubPublicKey    string `json:"hubPublicKey"`
	HubEndpoint     string `json:"hubEndpoint"`
	Peers           []struct{
		PublicKey string `json:"publicKey"`
		Address string `json:"address"`
		Endpoint string `json:"endpoint,omitempty"`
	} `json:"peers"`
}

func main() {
	hub := flag.String("hub", "http://127.0.0.1:8080", "Hub base URL")
	token := flag.String("token", "", "Enrollment token")
    noWG := flag.Bool("no-wg", false, "skip wireguard device configuration (for local testing)")
	flag.Parse()
	if *token == "" { log.Fatal("--token required") }

	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil { log.Fatal(err) }
	pub := priv.PublicKey().String()

	body, _ := json.Marshal(map[string]string{"token": *token, "publicKey": pub})
	resp, err := http.Post(*hub+"/api/register", "application/json", bytes.NewReader(body))
	if err != nil { log.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { b,_:=io.ReadAll(resp.Body); log.Fatalf("register failed %s %s", resp.Status, string(b)) }
	var reg registerResp
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil { log.Fatal(err) }
	log.Println("Assigned", reg.AssignedAddress, "HubPK", reg.HubPublicKey)

	if !*noWG {
		if err := ensureInterface(priv, reg); err != nil { log.Println("wg setup warn:", err) }

		for {
			// Keep-alive: reconfigure interface every 30s
			if err := refreshPeers(priv, reg); err != nil { log.Println("refresh warn:", err) }
			time.Sleep(30 * time.Second)
		}
	} else {
		log.Println("no-wg set: skipping wireguard setup. Registered OK:", reg.AssignedAddress)
		return
	}
}

func ensureInterface(priv wgtypes.Key, reg registerResp) error {
	c, err := wgctrl.New(); if err != nil { return err }
	defer c.Close()
	cfg := wgtypes.Config{PrivateKey: &priv}
	if err := c.ConfigureDevice("wg0", cfg); err != nil { return err }
	return nil
}

func refreshPeers(priv wgtypes.Key, reg registerResp) error {
	c, err := wgctrl.New(); if err != nil { return err }
	defer c.Close()
	peers := []wgtypes.PeerConfig{}
	for _, p := range reg.Peers {
		pubKey, err := wgtypes.ParseKey(p.PublicKey); if err != nil { continue }
		pc := wgtypes.PeerConfig{PublicKey: pubKey, PersistentKeepaliveInterval: durationPtr(15 * time.Second)}
		if p.Endpoint != "" {
			if ua, err := net.ResolveUDPAddr("udp", p.Endpoint); err == nil {
				pc.Endpoint = ua
			}
		}
		peers = append(peers, pc)
	}
	cfg := wgtypes.Config{Peers: peers}
	return c.ConfigureDevice("wg0", cfg)
}

func durationPtr(d time.Duration) *time.Duration { return &d }

func getEnv(k, def string) string { v:=os.Getenv(k); if v=="" {return def}; return v }

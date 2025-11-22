package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/suyogdahal/go-sdwan/shared"
)

var (
	regCounter = prometheus.NewCounter(prometheus.CounterOpts{Name: "sdwan_registrations_total", Help: "Total peer registrations"})
)

type registerReq struct {
	Token     string `json:"token"`
	PublicKey string `json:"publicKey"`
}

type registerResp struct {
	AssignedAddress string        `json:"assignedAddress"`
	HubPublicKey    string        `json:"hubPublicKey"`
	HubEndpoint     string        `json:"hubEndpoint"`
	Peers           []shared.Peer `json:"peers"`
}

func main() {
	noWG := flag.Bool("no-wg", false, "skip wireguard device configuration (for local testing)")
	flag.Parse()

	listen := getEnv("LISTEN", "0.0.0.0:8080")
	wgPort := getEnv("WG_PORT", "51820")
	state, err := shared.NewHubState()
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(regCounter)

	// Try to ensure interface exists (can skip with --no-wg for local testing)
	if !*noWG {
		if err := ensureInterface(state.HubPrivKey, wgPort); err != nil {
			log.Println("[warn] wireguard setup:", err)
		}
	} else {
		log.Println("Skipping wireguard interface setup (no-wg)")
	}

	http.HandleFunc("/api/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { w.WriteHeader(405); return }
		t := state.GenerateToken()
		json.NewEncoder(w).Encode(map[string]string{"token": t})
	})

	http.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { w.WriteHeader(405); return }
		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { w.WriteHeader(400); return }
		if !state.ValidateToken(req.Token) { w.WriteHeader(403); return }
		peer, err := state.AllocatePeer(req.PublicKey)
		if err != nil { w.WriteHeader(500); return }
		// observe remote endpoint (ip:port) and store it
		remote := r.RemoteAddr
		state.SetPeerEndpoint(req.PublicKey, remote)
		// compute hub endpoint host for clients (use Host header fallback)
		hostOnly := r.Host
		if strings.Contains(hostOnly, ":") {
			hostOnly = strings.Split(hostOnly, ":")[0]
		}
		hubEndpoint := fmt.Sprintf("%s:%s", hostOnly, wgPort)
		regCounter.Inc()
		resp := registerResp{AssignedAddress: peer.Address, HubPublicKey: state.HubPubKey}
		resp.HubEndpoint = hubEndpoint
		for _, p := range state.ListPeers() { resp.Peers = append(resp.Peers, *p) }
		json.NewEncoder(w).Encode(resp)
	})

	http.HandleFunc("/api/peers", func(w http.ResponseWriter, r *http.Request) {
		list := state.ListPeers()
		json.NewEncoder(w).Encode(list)
	})

	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		peers := state.ListPeers()
		sort.Slice(peers, func(i,j int) bool { return peers[i].LastSeen.After(peers[j].LastSeen) })
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><head><title>SD-WAN Hub</title><style>body{font-family:sans-serif}table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:4px}</style></head><body>")
		fmt.Fprintf(w, "<h1>go-sdwan Hub</h1><p>Hub Public Key: <code>%s</code></p>", state.HubPubKey)
		fmt.Fprintf(w, "<form method=post action=/api/token><button>Generate Enrollment Token</button></form>")
		fmt.Fprintf(w, "<h2>Peers (%d)</h2><table><tr><th>ID</th><th>Public Key</th><th>Address</th><th>Last Seen</th></tr>", len(peers))
		for _, p := range peers {
			fmt.Fprintf(w, "<tr><td>%s</td><td style='font-size:10px'>%s</td><td>%s</td><td>%s</td></tr>", p.ID, p.PublicKey, p.Address, time.Since(p.LastSeen).Round(time.Second))
		}
		fmt.Fprintf(w, "</table><p>Metrics: <a href=/metrics>/metrics</a></p></body></html>")
	})

	http.Handle("/metrics", promhttp.Handler())

	log.Println("Hub listening on", listen)
	log.Fatal(http.ListenAndServe(listen, loggingMiddleware(http.DefaultServeMux)))
}

func getEnv(k, def string) string { v:=os.Getenv(k); if v=="" {return def}; return v }

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w,r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func ensureInterface(priv wgtypes.Key, port string) error {
	c, err := wgctrl.New()
	if err != nil { return err }
	defer c.Close()
	// We rely on OS pre-created wg0 or external script; attempt configure only.
	cfg := wgtypes.Config{PrivateKey: &priv, ListenPort: intPtr(mustAtoi(port))}
	return c.ConfigureDevice("wg0", cfg)
}

func mustAtoi(s string) int { var n int; fmt.Sscanf(s, "%d", &n); return n }
func intPtr(i int) *int { return &i }

// AddPeer placeholder; real implementation would update kernel Peer config.
func addPeer(pub string, endpoint string) error {
	if !strings.Contains(endpoint, ":") { return fmt.Errorf("bad endpoint") }
	return nil
}

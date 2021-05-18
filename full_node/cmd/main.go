package main

import (
	"bufio"
	"container/list"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/full_node"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/visualize"
	"github.com/jroimartin/gocui"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

var (
	port       *string
	configPath *string
	keyPath    *string
	debugMode  *bool
)

func init() {
	port = flag.String("port", "10000", "port to listen to peers and wallet")
	configPath = flag.String("config_path", "full_node/cmd/config.yaml", "path to full node config")
	keyPath = flag.String("key_path", "/tmp/mykey.pem", "the path to read or write your credentials.")
	debugMode = flag.Bool("debug_mode", false, "Using debug mode will disable fancy GUI.")
}

// This function parses command from command line.
func ParseCommand(cmd chan commands.Command) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		// convert CRLF to LF
		text = strings.Replace(text, "\n", "", -1)
		c, err := commands.CreateCommand(text)
		if err != nil {
			log.Println(err)
			continue
		}
		cmd <- c
	}
}

// There are 2 commands supported:
// 1. Start/Stop mining.
// 2. Add/Remove peer.
func HandleCommand(cmd chan commands.Command, server *full_node.FullNodeServer) {
	// A separate control is needed to make sure cmd is non-blocking
	// when we just want to restart task.
	ctl := make(chan commands.Command, 1)
	is_mining := false
	is_probing := false
	for {
		c := <-cmd
		switch c.Op {
		case commands.START:
			if is_mining {
				server.Log("mining has already been started")
				continue
			}
			is_mining = true
			go func() {
				for {
					res, err := server.Mine(ctl)
					if err != nil {
						server.Log(err.Error())
					}
					if res.Op == commands.STOP {
						is_mining = false
						return
					}
				}
			}()
		case commands.RESTART, commands.STOP:
			if !is_mining {
				continue
			}
			go func() {
				// Relay the signal to mining process in a separate goroutine
				// because we don't want to block HandleCommand in any situation.
				ctl <- c
			}()
		case commands.ADD_PEER:
			self_addr := server.GetAddress()
			if self_addr.IpAddr == c.Args[0] && self_addr.Port == c.Args[1] {
				server.Log("cannot add self as peer")
				continue
			}
			// Add peer to be local client.
			err := server.AddMutualConnection(c.Args[0], c.Args[1])
			if err != nil {
				server.Log(fmt.Sprintf("cannot add new peer: %s", err.Error()))
			}
		case commands.LIST_PEER:
			for _, p := range server.GetAllPeers() {
				server.Log(p.String())
			}
		case commands.SHOW:
			v, err := strconv.Atoi(c.Args[0])
			if err != nil {
				server.Log(fmt.Sprintf("%s is not a valid number for depth", c.Args[0]))
			}
			server.Show(v)
		case commands.SYNC:
			go func() {
				err := server.SyncToLatest()
				if err != nil {
					server.Log(fmt.Sprintf("fail to sync to latest: " + err.Error()))
				}
			}()
		case commands.KEY:
			server.Log("\n===============DO NOT COPY THIS LINE================\n" + server.GetPublicKey() + "\n===============DO NOT COPY THIS LINE================")
		case commands.INTRODUCE:
			peers, err := server.Introduce(c.Args[0], c.Args[1])
			if err != nil {
				server.Log("fail to get introduced: " + err.Error())
			}
			s := c.Args[0] + ":" + c.Args[1] + " has peers ==>"
			for i := 0; i < len(peers); i++ {
				peer := peers[i]
				s = s + "\n" + peer.IpAddr + ":" + peer.Port
			}
			server.Log(s)
		case commands.NETWORK:
			go func() {
				if is_probing {
					server.Log("there's ongoing probing..")
					return
				}
				is_probing = true
				g := ProbNetwork(server)
				visualize.RenderGraph(g)
				is_probing = false
			}()
		default:
			server.Log(fmt.Sprintf("Unrecognized command: %d", c.Op))
		}
	}
}

// Prob the network in a BFS manner and construct the network graph.
func ProbNetwork(server *full_node.FullNodeServer) *visualize.Graph {
	seen := make(map[visualize.Address]bool)
	g := visualize.NewGraph()
	self := server.GetAddress()
	todo := list.New()
	todo.PushBack(visualize.NewAddress(self.IpAddr, self.Port))
	for todo.Len() != 0 {
		self_addr := todo.Front().Value.(visualize.Address)
		todo.Remove(todo.Front())
		if seen[self_addr] {
			continue
		}
		seen[self_addr] = true

		peer_addrs, err := server.Introduce(self_addr.Ip, self_addr.Port)
		if err != nil {
			continue
		}
		self := g.GetNode(self_addr)
		for i := 0; i < len(peer_addrs); i++ {
			peer := g.GetNode(visualize.NewAddress(peer_addrs[i].IpAddr, peer_addrs[i].Port))
			self.AddPeer(peer)
			todo.PushBack(visualize.NewAddress(peer_addrs[i].IpAddr, peer_addrs[i].Port))
		}
	}
	return g
}

func ParseAppConfig(path string) config.AppConfig {
	c := config.AppConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("yamlFile. get err: ", err.Error())
	}
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		log.Fatal("Unmarshal: ", err)
	}
	return c
}

// Get IPv4 public address
func localAddress() full_node.Address {
	/* DEPRECATED - IPv6 doesn't seem to be working :(
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalln(err)
	}

	// find a ipv6 global unicast address.
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() == nil && ipnet.IP.IsGlobalUnicast() {
			return full_node.Address{
				IpAddr: ipnet.IP.String(),
				Port:   *port,
			}
		}
	}

	log.Fatalln("doesn't find any ipv6 unicast address to listen to")
	return full_node.Address{}
	*/
	url := "https://api.ipify.org?format=text"

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	return full_node.Address{
		IpAddr: string(ip),
		Port:   *port,
	}
}

// Return a gui handle if not in debug mode.
func ListenOnInput(cmd chan commands.Command, debugMode bool) *gocui.Gui {
	// Choose a fancy GUI
	if debugMode {
		go ParseCommand(cmd)
	} else {
		g, err := CreateGui(cmd)
		if err != nil {
			log.Fatalln(err)
		}
		go func() {
			if err := g.MainLoop(); err != nil {
				if err == gocui.ErrQuit {
					g.Close()
					os.Exit(0)
				}
				os.Exit(1)
			}
		}()
		return g
	}
	return nil
}

func main() {
	flag.Parse()

	cfg := ParseAppConfig(*configPath)
	la := localAddress()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", la.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// A command channel that non-blockingly takes external or internal command
	// and handle it correspondingly.
	cmd := make(chan commands.Command)

	// Start listening on input.
	g := ListenOnInput(cmd, *debugMode)

	// Create a server with peer, config and a command channel to interrupt mining when tail changes.
	server := full_node.NewFullNodeServer(cfg, []full_node.Peer{}, localAddress(), *keyPath, cmd, g)
	grpcServer := grpc.NewServer()
	service.RegisterFullNodeServiceServer(grpcServer, server)

	// Create 2 routine dedicated for mining.
	// cmd: Parse string input and create command.
	// ctl: A separate channel that pass signal to mining routine to interrupt the mining process.
	// ctl needs to be passed to server in order to let each

	go HandleCommand(cmd, server)

	server.Log(fmt.Sprintf("Starting to serve at endpoint: %s:%s", la.IpAddr, la.Port))
	grpcServer.Serve(lis)
}

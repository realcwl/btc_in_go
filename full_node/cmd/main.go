package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/full_node"
	"github.com/Luismorlan/btc_in_go/service"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

var (
	port       *string
	peers      *string
	peerPorts  *string
	configPath *string
)

func init() {
	port = flag.String("port", "10000", "port to listen to peers and wallet")
	peers = flag.String("peers", "", "peer ip addresses")
	peerPorts = flag.String("peer_ports", "", "peer ports")
	configPath = flag.String("config_path", "full_node/cmd/config.yaml", "path to full node config")
}

var iter = 0

func IntensiveCompute(ctl chan commands.Command) commands.Command {
	iter++
	log.Println("iteration: ", iter)
	for i := 0; i <= 5; i++ {
		select {
		case c := <-ctl:
			return c
		default:
			log.Println(i)
			time.Sleep(1 * time.Second)
		}
	}
	log.Println("All task done, return")
	return commands.Command{Op: commands.RESTART}
}

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
	is_running := false
	for {
		c := <-cmd
		switch c.Op {
		case commands.START:
			if is_running {
				log.Print("mining has already been started\n> ")
				continue
			}
			is_running = true
			go func() {
				for {
					res, err := server.Mine(ctl)
					if err != nil {
						log.Print(err)
						fmt.Print("> ")
					}
					if res.Op == commands.STOP {
						is_running = false
						return
					}
				}
			}()
		case commands.RESTART, commands.STOP:
			if !is_running {
				log.Print("no running mining task to be restart or shut")
				fmt.Print("> ")
				continue
			}
			go func() {
				// Relay the signal to mining process in a separate goroutine
				// because we don't want to block HandleCommand in any situation.
				ctl <- c
			}()
		case commands.SHOW:
			v, err := strconv.Atoi(c.Args[0])
			if err != nil {
				log.Printf("%s is not a valid number for depth", c.Args[0])
			}
			server.Show(v)
		default:
			log.Print("Unrecognized command:", c)
			fmt.Print("> ")
		}
	}
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

func main() {
	flag.Parse()

	cfg := ParseAppConfig(*configPath)

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Println(*port, *peers, *peerPorts)

	// A command channel that non-blockingly takes external or internal command
	// and handle it correspondingly.
	cmd := make(chan commands.Command)

	// Create a server with peer, config and a command channel to interrupt mining when tail changes.
	server := full_node.NewFullNodeServer(cfg, []full_node.Peer{}, cmd)
	grpcServer := grpc.NewServer()
	service.RegisterFullNodeServiceServer(grpcServer, server)
	log.Println(cfg)
	log.Println("Starting to serve at port:", *port)

	// Create 2 routine dedicated for mining.
	// cmd: Parse string input and create command.
	// ctl: A separate channel that pass signal to mining routine to interrupt the mining process.
	// ctl needs to be passed to server in order to let each
	go ParseCommand(cmd)
	go HandleCommand(cmd, server)

	grpcServer.Serve(lis)
}

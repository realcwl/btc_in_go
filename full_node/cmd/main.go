package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/full_node"
	"github.com/Luismorlan/btc_in_go/service"
	"google.golang.org/grpc"
)

var (
	port       string
	peers      string
	peer_ports string
)

func init() {
	port = *flag.String("port", "10000", "port to listen to peers and wallet")
	peers = *flag.String("peers", "", "peer ip addresses")
	peer_ports = *flag.String("peer_ports", "", "peer ports")
}

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := full_node.NewFullNodeServer(config.AppConfig{}, []full_node.Peer{})

	grpcServer := grpc.NewServer()
	service.RegisterFullNodeServiceServer(grpcServer, server)
	log.Println("Starting to serve at port:", port)
	grpcServer.Serve(lis)
}

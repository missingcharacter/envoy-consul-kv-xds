package main

import (
	"fmt"
	"net"
	"os"
	"sync"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache"
	xds "github.com/envoyproxy/go-control-plane/pkg/server"
	flags "github.com/jessevdk/go-flags"
	consul "github.com/missingcharacter/envoy-consul-kv-xds/consul"
	resources "github.com/missingcharacter/envoy-consul-kv-xds/resources"
	logging "github.com/op/go-logging"
	"google.golang.org/grpc"
)

type hash struct{}

const (
	defaultNode = "default_node"
	// AppName is the name of this application
	appName = "consul-xds"
	// AppVersion is the version of this application
	appVersion = "0.0.1"
)

var (
	log    = logging.MustGetLogger(appName)
	format = logging.MustStringFormatter(
		`%{color}%{level:-7s}: %{time} %{shortfile} %{longfunc} %{id:03x}%{color:reset} %{message}`,
	)
	opts struct {
		Verbose           bool     `short:"v" long:"verbose" description:"Enable DEBUG logging"`
		DoVersion         bool     `short:"V" long:"version" description:"Print version and exit"`
		ConsulURL         string   `short:"u" long:"consul-url" description:"Consul URL (including host and port, e.g. '127.0.0.1:8500')" default:"" env:"CONSUL_URL"`
		ConsulSSL         bool     `short:"s" long:"consul-ssl" description:"If set will instruct communications to the Consul API to be done via SSL." env:"CONSUL_SSL"`
		ServicesNamespace string   `short:"n" long:"services-namespace" description:"Consul KV namespace to loook for services configuration." default:"service" env:"SERVICES_NAMESPACE"`
		ServiceFilters    []string `short:"f" long:"service-filters" description:"Names of keys where to look for services configuration." default:"public,private" env-delim:"," env:"SERVICE_FILTERS"`
		ServicesHealth    string   `short:"h" long:"services-health" description:"Name of the key where to look for the services health configuration." default:"health" env:"SERVICES_HEALTH"`
		XDSAddr           string   `short:"x" long:"xds-addr" description:"The address on which to serve the envoy API server, e.g. ':50000' or '127.0.0.1:50000'" default:":50000" env:"XDS_ADDR"`
	}
	clusters, endpoints, routes, listeners []cache.Resource
)

func (hash) ID(node *core.Node) string {
	if node != nil {
		return node.Id
	}
	return defaultNode
}

func main() {
	// Parse arguments
	_, err := flags.Parse(&opts)
	if err != nil {
		typ := err.(*flags.Error).Type
		if typ == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Print version number if requested from command line
	if opts.DoVersion == true {
		fmt.Printf("%s %s at your service.\n", appName, appVersion)
		os.Exit(10)
	}

	// Configure logger
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(logBackend, format)
	logging.SetBackend(backendFormatter)

	// Enable debug logging
	if opts.Verbose == true {
		logging.SetLevel(logging.DEBUG, "")
	} else {
		logging.SetLevel(logging.INFO, "")
	}

	log.Debugf("Commandline options: %+v", opts)

	// TODO: replace initial empty snapshot with functional one
	snapshotCache := cache.NewSnapshotCache(false, hash{}, log)
	server := xds.NewServer(snapshotCache, nil)

	// Set initial cache
	snapshot := cache.NewSnapshot("1.0", endpoints, clusters, routes, listeners)
	_ = snapshotCache.SetSnapshot("", snapshot)

	// Create server
	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", opts.XDSAddr)
	if err != nil {
		log.Fatalf("Error in listener: %v", err)
	}

	// Register routes
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	v2.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	v2.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	v2.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	v2.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	// TODO: add web api for dumping config/realtime adjustments?

	// Make a consul client
	client, err := consul.Config{
		Host:   opts.ConsulURL,
		UseSSL: opts.ConsulSSL,
	}.GetClient()
	if err != nil {
		log.Fatalf("Got error: [%v]", err)
	}

	log.Info("Starting the server...")

	_ = snapshotCache.SetSnapshot("", resources.Config{
		Namespace: opts.ServicesNamespace,
		Filters:   opts.ServiceFilters,
		Health:    opts.ServicesHealth,
	}.MakeSnapshot(client))

	// Start the server
	// TODO: start consul monitor thread here too
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcServer.Serve(lis); err != nil {
			// error handling
			log.Fatalf("Got error: [%v]\n", err)
		} else {
			log.Info("Server finished.")
		}
	}()
	wg.Wait()
}

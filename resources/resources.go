package resources

import (
	"fmt"
	"strings"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/test/resource"
	"github.com/gogo/protobuf/types"
	consulapi "github.com/hashicorp/consul/api"
	consul "github.com/missingcharacter/envoy-consul-kv-xds/consul"
	logging "github.com/op/go-logging"
)

var log = logging.MustGetLogger("resources")

// Config represents the necessary context to find services configuration
// in consul KV
type Config struct {
	Namespace string
	Filters   []string
	Health    string
}

// MakeSnapshot creates the initial cache.Snapshot with all necessary
// context for envoy to find services and routes
func (config Config) MakeSnapshot(c consul.ConsulClient) cache.Snapshot {
	log.Info("Creating Snapshot")
	var (
		clusters, endpoints, routes, listeners []cache.Resource
		lbEndpoints                            []endpoint.LbEndpoint
		filters                                []string
		metadata                               core.Metadata
	)
	vHosts := make(map[string][]route.VirtualHost)
	//serviceVirtualHosts := make([]route.VirtualHost, 0)

	qo := &consulapi.QueryOptions{RequireConsistent: true}

	// Clusters (aka Consul Services)
	services, _, _ := c.Catalog().Services(qo)

	// Download consul KV
	kvs, _, _ := c.KV().List(config.Namespace, qo)
	// Filter KV to services we know about
	filters = append(config.Filters, config.Health)
	filteredKvs := consul.GetServicesKVs(kvs, filters)

	for service := range services {
		log.Debugf("Discovered service: [%s]\n", service)
		lbEndpoints = nil
		metadata = core.Metadata{}
		clusters = append(clusters, resource.MakeCluster(resource.Xds, service))

		// Endpoints (aka Consul Service Entries?)
		eps, _, _ := c.Health().Service(service, "", true, qo)
		for _, ep := range eps {
			if ep.Service.Port == 0 || ep.Node.TaggedAddresses["wan"] == "" {
				log.Warningf("Skipping [%s] since service port or address not set!\n", service)
				continue
			}
			metadata = core.Metadata{
				FilterMetadata: map[string]*types.Struct{
					"envoy.lb": {
						Fields: map[string]*types.Value{},
					},
				},
			}
			metadata.FilterMetadata["envoy.lb"].Fields["node-id"] = &types.Value{Kind: &types.Value_StringValue{StringValue: ep.Node.ID}}
			metadata.FilterMetadata["envoy.lb"].Fields["node-name"] = &types.Value{Kind: &types.Value_StringValue{StringValue: ep.Node.Node}}
			metadata.FilterMetadata["envoy.lb"].Fields["datacenter"] = &types.Value{Kind: &types.Value_StringValue{StringValue: ep.Node.Datacenter}}
			if len(ep.Service.Tags) > 0 {
				var tags []*types.Value
				for _, tag := range ep.Service.Tags {
					tags = append(tags, &types.Value{Kind: &types.Value_StringValue{StringValue: tag}})
				}
				metadata.FilterMetadata["envoy.lb"].Fields["tags"] = &types.Value{Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: tags}}}
			}
			if ep.Service.Address != "" {
				metadata.FilterMetadata["envoy.lb"].Fields["svc-address"] = &types.Value{Kind: &types.Value_StringValue{StringValue: ep.Service.Address}}
			}
			if ep.Node.TaggedAddresses["lan"] != "" {
				metadata.FilterMetadata["envoy.lb"].Fields["lan-address"] = &types.Value{Kind: &types.Value_StringValue{StringValue: ep.Node.TaggedAddresses["lan"]}}
			}
			if ep.Node.TaggedAddresses["wan"] != "" {
				metadata.FilterMetadata["envoy.lb"].Fields["wan-address"] = &types.Value{Kind: &types.Value_StringValue{StringValue: ep.Node.TaggedAddresses["wan"]}}
			}
			log.Debugf("Adding endpoint [%s]: %s:%d", service, ep.Node.TaggedAddresses["wan"], ep.Service.Port)
			lbEndpoints = append(lbEndpoints, MakeEndpoint(ep.Node.TaggedAddresses["wan"], uint32(ep.Service.Port), 1, metadata))
		}

		if len(lbEndpoints) > 0 {
			endpoints = append(endpoints, MakeEndpoints(service, lbEndpoints))
		}

		if len(config.Filters) > 0 {
			for _, filter := range config.Filters {
				// Routes (aka Consul KV)
				domainKVs := consul.GetServiceKVs(filteredKvs[filter], service)
				domains := make([]string, 0)
				if filter == "private" {
					domains = append(domains, fmt.Sprintf("%s.internal", service))
				}
				for _, kv := range domainKVs {
					log.Debugf("Discovered route for service: [%s] at KV: [%s], [%s]", service, kv.Key, kv.Value)

					// TODO: ensure KV contains valid domain, and isn't haproxy specific!
					if strings.Contains(string(kv.Value), " ") {
						log.Debugf("[%s] is not a valid domain, therefore it will not be added", kv.Value)
					} else {
						domains = append(domains, string(kv.Value))
					}
				}

				if len(domains) > 0 {
					log.Debugf("Adding routes for service [%s] : %v", service, domains)
					vHosts[filter] = append(vHosts[filter], MakeVirtualHost(service, domains, filter))
					routes = append(routes, MakeRoutes(filter, vHosts[filter]))
				}
			}
		}

	}

	log.Info("Finished Snapshot")
	return cache.NewSnapshot("1.0", endpoints, clusters, routes, listeners)
}

//func (c ccAdapter) makeEndpoints() []cache.Resource {
//	var endpoints []cache.Resource
//
//	endpoints, _, _ := c.underlying.Catalog().Services(&consulapi.QueryOptions{RequireConsistent: true})
//	for service := range endpoints {
//		endpoints = append(endpoints, resource.MakeEndpoint(resource.Xds, service))
//	}
//
//}

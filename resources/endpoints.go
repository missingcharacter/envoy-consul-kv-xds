package resources

import (
	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
)

// MakeEndpoints creates a cluster with all its endpoints
func MakeEndpoints(clusterName string, eps []endpoint.LbEndpoint) *v2.ClusterLoadAssignment {
	return &v2.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []endpoint.LocalityLbEndpoints{{
			LbEndpoints: eps,
		}},
	}
}

// MakeEndpoint creates an endpoint on a given port with its current health status
// and context information as metadata
func MakeEndpoint(host string, port uint32, healthStatus core.HealthStatus, metadata core.Metadata) endpoint.LbEndpoint {
	return endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: &core.Address{
					Address: &core.Address_SocketAddress{
						SocketAddress: &core.SocketAddress{
							Protocol: core.TCP,
							Address:  host,
							PortSpecifier: &core.SocketAddress_PortValue{
								PortValue: port,
							},
						},
					},
				},
			},
		},
		HealthStatus: healthStatus,
		Metadata:     &metadata,
	}
}

// GenerateEndpoints returns all endpoints for this node and all other endpoints
/*func GenerateEndpoints(node string) ([]endpoint.LocalityLbEndpoints, []endpoint.LocalityLbEndpoints) {
	// result of "http://consul-master:8500/v1/catalog/node/"+node
	// Example: http://consul-master:8500/v1/catalog/node/instance-hostname
	myServices := "stuff"
	allServices := "All stuff"
	return myServices, allServices
}*/

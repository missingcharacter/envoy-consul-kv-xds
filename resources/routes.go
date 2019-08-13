package resources

import (
	"fmt"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
)

// MakeRoute creates an HTTP route that routes to a given cluster.
//type clusterName string
//type routeConfig struct{
//	clusterName []string,
//	domains []string,
//}
//type routerConfig map[clusterName]routeConfig

// MakeRoutes creates all routes available to a listener
func MakeRoutes(listenerName string, vhosts []route.VirtualHost) *v2.RouteConfiguration {
	return &v2.RouteConfiguration{
		Name:         listenerName,
		VirtualHosts: vhosts,
	}
}

// MakeVirtualHost creates a virtual host named `cluster-listener` with all domains
// accepted in `Host:` header  and routes to clusters
func MakeVirtualHost(clusterName string, domains []string, listenerName string) route.VirtualHost {
	return route.VirtualHost{
		Name:    fmt.Sprintf("%s-%s", clusterName, listenerName),
		Domains: domains,
		Routes: []route.Route{{
			Match: route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: clusterName,
					},
				},
			},
		},
		}}
}

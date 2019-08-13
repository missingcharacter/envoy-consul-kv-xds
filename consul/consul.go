package consul

import (
	"strings"

	consulapi "github.com/hashicorp/consul/api"
)

type ConsulClient struct {
	underlying *consulapi.Client
}

type Config struct {
	UseSSL bool
	Host   string
}

func (c ConsulClient) Catalog() *consulapi.Catalog {
	return c.underlying.Catalog()
}

func (c ConsulClient) KV() *consulapi.KV {
	return c.underlying.KV()
}

func (c ConsulClient) Health() *consulapi.Health {
	return c.underlying.Health()
}

func (c Config) GetClient() (ConsulClient, error) {
	scheme := "http"
	if c.UseSSL {
		scheme = "https"
	}

	cfg := &consulapi.Config{
		Address: c.Host,
		Scheme:  scheme,
	}

	client, e := consulapi.NewClient(cfg)
	return ConsulClient{client}, e
}

func GetServicesKVs(kvp consulapi.KVPairs, filters []string) map[string]consulapi.KVPairs {
	result := make(map[string]consulapi.KVPairs, 0)
	for _, kv := range kvp {
		pathSplit := strings.Split(kv.Key, "/")
		for _, filter := range filters {
			if strings.Contains(pathSplit[len(pathSplit)-1], filter) {
				result[filter] = append(result[filter], kv)
			}
		}
	}
	return result
}

func GetServiceKVs(kvp consulapi.KVPairs, service string) consulapi.KVPairs {
	result := make(consulapi.KVPairs, 0)
	for _, kv := range kvp {
		pathSplit := strings.Split(kv.Key, "/")
		if pathSplit[1] == service {
			result = append(result, kv)
		}
	}
	return result
}

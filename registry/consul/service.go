package consul

import (
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// watchServices monitors the consul health checks and creates a new configuration
// on every change.
func watchServices(client *api.Client, tagPrefix string, status []string, config chan string, dcIndex int, datacenters []string) {
	var lastIndex uint64

	for {
		var all_checks []*api.HealthCheck
		q := &api.QueryOptions{RequireConsistent: true, WaitIndex: lastIndex, Datacenter: datacenters[dcIndex]}
		checks, meta, err := client.Health().State("any", q)
		if err != nil {
			log.Printf("[WARN] consul: Error fetching health state. %v", err)
			time.Sleep(time.Second)
			continue
		}
		for _, check := range checks {
			all_checks = append(all_checks, check)
		}
		log.Printf("[WARN] consul: Health changed to #%d", meta.LastIndex)
		lastIndex = meta.LastIndex
		for i, dc := range datacenters {
			if i == dcIndex {
				continue
			}

			q := &api.QueryOptions{RequireConsistent: true, Datacenter: dc}
			checks, _, err := client.Health().State("any", q)
			if err != nil {
				log.Printf("[WARN] consul: Error fetching health state. %v", err)
				time.Sleep(time.Second)
				continue
			}
			for _, check := range checks {
				all_checks = append(all_checks, check)
			}
		}
		config <- servicesConfig(client, passingServices(all_checks, status), tagPrefix)
	}
}

// servicesConfig determines which service instances have passing health checks
// and then finds the ones which have tags with the right prefix to build the config from.
func servicesConfig(client *api.Client, checks []*api.HealthCheck, tagPrefix string) string {
	// map service name to list of service passing for which the health check is ok
	m := map[string]map[string]bool{}
	for _, check := range checks {
		name, id := check.ServiceName, check.ServiceID

		if _, ok := m[name]; !ok {
			m[name] = map[string]bool{}
		}
		m[name][id] = true
	}

	var config []string
	for name, passing := range m {
		cfg := serviceConfig(client, name, passing, tagPrefix)
		config = append(config, cfg...)
	}

	// sort config in reverse order to sort most specific config to the top
	sort.Sort(sort.Reverse(sort.StringSlice(config)))

	return strings.Join(config, "\n")
}

// serviceConfig constructs the config for all good instances of a single service.
func serviceConfig(client *api.Client, name string, passing map[string]bool, tagPrefix string) (config []string) {
	if name == "" || len(passing) == 0 {
		return nil
	}

	datacenters, err := client.Catalog().Datacenters()
	if err != nil {
		log.Printf("[WARN] consul: Error getting datacenters. %s", err)
		return nil
	}

	for _, dc := range datacenters {
		q := &api.QueryOptions{RequireConsistent: true, Datacenter: dc}
		svcs, _, err := client.Catalog().Service(name, "", q)
		if err != nil {
			log.Printf("[WARN] consul: Error getting catalog service %s. %v", name, err)
			return nil
		}

		env := map[string]string{
			"DC": dc,
		}

		for _, svc := range svcs {
			// check if the instance is in the list of instances
			// which passed the health check
			if _, ok := passing[svc.ServiceID]; !ok {
				continue
			}

			for _, tag := range svc.ServiceTags {
				if host, path, ok := parseURLPrefixTag(tag, tagPrefix, env); ok {
					name, addr, port := svc.ServiceName, svc.ServiceAddress, svc.ServicePort

					// use consul node address if service address is not set
					if addr == "" {
						addr = svc.Address
					}

					// add .local suffix on OSX for simple host names w/o domain
					if runtime.GOOS == "darwin" && !strings.Contains(addr, ".") && !strings.HasSuffix(addr, ".local") {
						addr += ".local"
					}

					config = append(config, fmt.Sprintf("route add %s %s%s http://%s:%d/ tags %q", name, host, path, addr, port, strings.Join(svc.ServiceTags, ",")))
				}
			}
		}
	}
	return config
}

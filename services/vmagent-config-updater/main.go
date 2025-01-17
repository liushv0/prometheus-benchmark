package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	listenAddr                 = flag.String("httpListenAddr", ":8436", "TCP address for incoming HTTP requests")
	targetsCount               = flag.Int("targetsCount", 100, "The number of scrape targets to return from -httpListenAddr. Each target has the same address defined by -targetAddr")
	targetAddr                 = flag.String("targetAddr", "demo.robustperception.io:9090", "Address with port to use as target address the scrape config returned from -httpListenAddr")
	scrapeInterval             = flag.Duration("scrapeInterval", time.Second*5, "The scrape_interval to set at the scrape config returned from -httpListenAddr")
	scrapeConfigUpdateInterval = flag.Duration("scrapeConfigUpdateInterval", time.Minute*10, "The -scrapeConfigUpdatePercent scrape targets are updated in the scrape config returned from -httpListenAddr every -scrapeConfigUpdateInterval")
	scrapeConfigUpdatePercent  = flag.Float64("scrapeConfigUpdatePercent", 1, "The -scrapeConfigUpdatePercent scrape targets are updated in the scrape config returned from -httpListenAddr ever -scrapeConfigUpdateInterval")
	metricsPath                = flag.String("metricsPath", "/metrics", "The HTTP resource path on which to fetch metrics from targets. /metrics for default")
	scheme                     = flag.String("scheme", "http", "Configures the protocol scheme used for requests.  default = http")
)

func main() {
	flag.Parse()
	flag.VisitAll(func(f *flag.Flag) {
		log.Printf("-%s=%s", f.Name, f.Value)
	})
	c := newConfig(*targetsCount, *scrapeInterval, *targetAddr, *metricsPath, *scheme)
	var cLock sync.Mutex
	p := *scrapeConfigUpdatePercent / 100
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	go func() {
		rev := 0
		for range time.Tick(*scrapeConfigUpdateInterval) {
			rev++
			revStr := fmt.Sprintf("r%d", rev)
			cLock.Lock()
			for _, sc := range c.ScrapeConfigs {
				for _, stc := range sc.StaticConfigs {
					if r.Float64() >= p {
						continue
					}
					stc.Labels["churnLabel"] = revStr
				}
			}
			cLock.Unlock()
		}
	}()
	rh := func(w http.ResponseWriter, r *http.Request) {
		cLock.Lock()
		data := c.marshalYAML()
		cLock.Unlock()
		w.Header().Set("Content-Type", "text/yaml")
		w.Write(data)
	}
	hf := http.HandlerFunc(rh)
	log.Printf("starting scrape config updater at http://%s/", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, hf); err != nil {
		log.Fatalf("unexpected error when running the http server: %s", err)
	}
}

func (c *config) marshalYAML() []byte {
	data, err := yaml.Marshal(c)
	if err != nil {
		log.Fatalf("BUG: unexpected error when marshaling config: %s", err)
	}
	return data
}

func newConfig(targetsCount int, scrapeInterval time.Duration, targetAddr string, metricsPath string, scheme string) *config {
	scs := make([]*staticConfig, 0, targetsCount)
	for i := 0; i < targetsCount; i++ {
		scs = append(scs, &staticConfig{
			Targets: []string{targetAddr},
			Labels: map[string]string{
				"instance":   fmt.Sprintf("host-%d", i),
				"churnLabel": "r0",
			},
		})
	}
	return &config{
		Global: globalConfig{
			ScrapeInterval: scrapeInterval,
		},
		ScrapeConfigs: []*scrapeConfig{
			{
				JobName:       "node_exporter",
				Scheme:        scheme,
				MetricsPath:   metricsPath,
				StaticConfigs: scs,
			},
		},
	}
}

// config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type config struct {
	Global        globalConfig    `yaml:"global"`
	ScrapeConfigs []*scrapeConfig `yaml:"scrape_configs"`
}

// globalConfig represents essential parts for `global` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type globalConfig struct {
	ScrapeInterval time.Duration `yaml:"scrape_interval"`
}

// rapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type scrapeConfig struct {
	JobName       string          `yaml:"job_name"`
	MetricsPath   string          `yaml:"metrics_path"`
	Scheme        string          `yaml:"scheme"`
	StaticConfigs []*staticConfig `yaml:"static_configs"`
}

// staticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type staticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

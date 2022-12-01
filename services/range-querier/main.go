package main

import (
	"flag"
	"fmt"
	"github.com/cespare/xxhash/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	url2 "net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	datasource      = flag.String("datasource.url", "", "datasource")
	queryInterval   = flag.Duration("queryInterval", time.Second*10, " query Interval ")
	listenAddr      = flag.String("httpListenAddr", ":9234", "metrics port")
	remoteName      = flag.String("remoteStorage", "", "remote name")
	promqlFile      = flag.String("promqlFile", "", "range query promql list")
	readBearerToken = flag.String("datasource.bearerToken", "", "read bearer token")
	readHeaders     = flag.String("remoteRead.headers", "", "read ext headers")
)
var (
	query_cnt = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "prom_range_query_query_cnt",
		Help: "range query query cnt",
	}, []string{"remote_storage", "query_range", "promql", "status_code"})
	query_failed_cnt = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "prom_failed_range_query_cnt",
		Help: "failed range query cnt",
	}, []string{"remote_storage", "query_range", "promql", "status_code"})
	rt_histogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "prom_range_query_rt_ms",
		Help:    "prom range query rt histogram, millisecond",
		Buckets: []float64{100, 500, 1000, 3000, 10000},
	}, []string{"remote_storage", "query_range", "promql", "status_code"})
)

var query_url_format = "%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%d"

func main() {
	flag.Parse()
	flag.VisitAll(func(f *flag.Flag) {
		log.Printf("-%s=%s", f.Name, f.Value)
	})
	log.Printf("datasource url: %s, query interval: %d, remote name: %s, query file: %s", *datasource, *queryInterval, *remoteName, *promqlFile)

	if len(*datasource) == 0 || len(*promqlFile) == 0 {
		log.Printf("invalid params: datasource url or promqlFile can not be empty")
		return
	}

	cont, err := ioutil.ReadFile(*promqlFile)
	if err != nil || len(cont) == 0 {
		fmt.Println("read promql file failed, or empty file", err)
		panic(err)
	}
	prom_list := &query_statement_list{}
	err = yaml.Unmarshal(cont, prom_list)
	if err != nil {
		fmt.Println("can not unmarshal promql file, file content:")
		fmt.Println(string(cont))
		panic(err)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(*listenAddr, nil)
	}()

	var headers = make(map[string][]string)
	if len(*readHeaders) > 0 {
		header_list := strings.Split(*readHeaders, "^^")
		for _, h := range header_list {
			h_split := strings.Split(h, ":")
			if len(h_split) == 2 {
				k := strings.TrimSpace(h_split[0])
				v := strings.TrimSpace(h_split[1])
				if len(k) > 0 && len(v) > 0 {
					headers[k] = []string{v}
				}
			}
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	res_ch := make(chan query_res, 200)
	go func() {
		tick := time.NewTicker(*queryInterval)
		for {
			select {
			case res := <-res_ch:
				labels := make(map[string]string)
				labels["remote_storage"] = *remoteName
				labels["query_range"] = strconv.Itoa(res.query_range)
				labels["promql"] = strconv.FormatUint(xxhash.Sum64String(res.promql), 10)
				labels["status_code"] = strconv.Itoa(res.res_status)
				if res.success {
					query_cnt.With(labels).Inc()
					rt_histogram.With(labels).Observe(res.rt)
				} else {
					query_failed_cnt.With(labels).Inc()
				}
			case <-tick.C:
				range_query(wg, res_ch, prom_list, headers)
			}
		}
	}()

	wg.Wait()
}

func range_query(wg sync.WaitGroup, res_ch chan query_res, prom_list *query_statement_list, headers http.Header) {
	now := time.Now()
	end := now.Unix()
	client := http.Client{
		Timeout: time.Second * 30,
	}

	for _, promql := range prom_list.PromqlList {
		go func(wg sync.WaitGroup, promql query_statement, res_ch chan query_res, headers http.Header) {
			wg.Add(1)
			defer wg.Done()

			start := now.Add(promql.Query_range * -1).Unix()
			step := int(promql.Query_step.Seconds())

			raw_url := fmt.Sprintf(query_url_format, *datasource, url2.QueryEscape(promql.Promql), start, end, step)
			url, err := url2.ParseRequestURI(raw_url)
			if err != nil {
				fmt.Println("url parse error", err)
				return
			}
			req := &http.Request{
				URL:    url,
				Method: "GET",
			}
			if len(headers) > 0 {
				req.Header = headers
			}

			begin := time.Now().UnixNano()
			resp, err := client.Do(req)
			finish := time.Now().UnixNano()

			res := query_res{
				success:     err == nil,
				promql:      promql.Promql,
				query_range: int(promql.Query_range.Hours()),
				res_status:  resp_status(resp),
				rt:          float64((finish - begin) / time.Millisecond.Nanoseconds()),
			}
			res_ch <- res
		}(wg, promql, res_ch, headers)
	}
}

func resp_status(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

type query_res struct {
	success     bool
	promql      string
	query_range int
	res_status  int
	rt          float64
}

type query_statement struct {
	Promql      string        `yaml:"promql"`
	Query_range time.Duration `yaml:"query_range"`
	Query_step  time.Duration `yaml:"query_step"`
}

type query_statement_list struct {
	PromqlList []query_statement `yaml:"promql_list"`
}

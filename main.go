package main

import (
	"log"
	"net/http"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var listenFlag = kingpin.Flag("listen", "address on which to expose the exporter").Default(":10018").String()
var mountpointsFlag = kingpin.Flag("mountpoint", "path to the mountpoints whose quotas should be exported").Required().Strings()

func main() {
	kingpin.Parse()

	prometheus.MustRegister(NewQuotaCollector(*mountpointsFlag))
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("Serving metrics on %v", *listenFlag)
	err := http.ListenAndServe(*listenFlag, nil)
	log.Fatal(err)
}

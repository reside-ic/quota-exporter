package main

import (
	"log"
	"net/http"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var listenFlag = kingpin.Flag("listen", "address on which to expose the exporter").Default(":10018").String()
var mountpointsFlag = kingpin.Flag("mountpoint", "path to the mountpoints whose quotas should be exported").Strings()
var allFlag = kingpin.Flag("all", "export data for all mount points with quotas").Bool()

func main() {
	kingpin.Parse()

	if len(*mountpointsFlag) > 0 && *allFlag {
		kingpin.Fatalf("--mountpoint and --all are mutually exclusive")
	} else if len(*mountpointsFlag) == 0 && (!*allFlag) {
		kingpin.Fatalf("Either --mountpoint or --all must be provided")
	}

	prometheus.MustRegister(version.NewCollector("quota_exporter"))
	prometheus.MustRegister(NewQuotaCollector(*allFlag, *mountpointsFlag))
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("Serving metrics on %v", *listenFlag)
	err := http.ListenAndServe(*listenFlag, nil)
	log.Fatal(err)
}
